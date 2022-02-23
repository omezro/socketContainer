package main

import (
  "encoding/json"
  "fmt"
  "net/http"
  "socketContainer/common"
  "socketContainer/ws"

  "github.com/gorilla/websocket"
  "k8s.io/api/core/v1"
  "k8s.io/client-go/kubernetes"
  "k8s.io/client-go/kubernetes/scheme"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/remotecommand"
)

var clientset *kubernetes.Clientset

type streamHandler struct {
  wsConn      *ws.WsConnection
  resizeEvent chan remotecommand.TerminalSize
}

type xtermMessage struct {
  MsgType string `json:"type"`
  Input   string `json:"input"`
  Rows    uint16 `json:"rows"`
  Cols    uint16 `json:"cols"`
}

func (handler *streamHandler) Next() (size *remotecommand.TerminalSize) {
  ret := <-handler.resizeEvent
  size = &ret
  return
}

func (handler *streamHandler) Read(p []byte) (size int, err error) {
  var (
    msg      *ws.WsMessage
    xtermMsg xtermMessage
  )
  if msg, err = handler.wsConn.WsRead(); err != nil {
    return
  }

  if err = json.Unmarshal(msg.Data, &xtermMsg); err != nil {
    return
  }
  if xtermMsg.MsgType == "resize" {
    handler.resizeEvent <- remotecommand.TerminalSize{
      Width:  xtermMsg.Cols,
      Height: xtermMsg.Rows,
    }
  } else if xtermMsg.MsgType == "input" {
    size = len(xtermMsg.Input)
    copy(p, []byte(xtermMsg.Input))
  }
  return
}

func (handler *streamHandler) Write(p []byte) (size int, err error) {
  var copyData []byte

  copyData = make([]byte, len(p))
  copy(copyData, p)
  size = len(p)
  err = handler.wsConn.WsWrite(websocket.TextMessage, copyData)
  return
}

func wsHandler(resp http.ResponseWriter, req *http.Request) {
  var (
    wsConn        *ws.WsConnection
    restConf      *rest.Config
    sshReq        *rest.Request
    podName       string
    podNs         string
    containerName string
    executor      remotecommand.Executor
    handler       *streamHandler
    err           error
  )

  if err = req.ParseForm(); err != nil {
    return
  }

  podNs = req.Form.Get("podNs")
  podName = req.Form.Get("podName")
  containerName = req.Form.Get("containerName")

  if wsConn, err = ws.InitWebsocket(resp, req); err != nil {
    return
  }

  podName = "redis-a-zs2f7-0"
  podNs = "lingfang-8286af39-cab6-4363-b89d-39ce951d2b71"
  containerName = "redis-a-zs2f7"

  if restConf, err = common.GetRestConf(); err != nil {
    goto END
  }

  sshReq = clientset.
    CoreV1().
    RESTClient().
    Post().
    Resource("pods").
    Name(podName).
    Namespace(podNs).
    SubResource("exec").
      VersionedParams(&v1.PodExecOptions{
        Container: containerName,
        Command:   []string{"sh"},
        Stdin:     true,
        Stdout:    true,
        Stderr:    true,
        TTY:       true,
      }, scheme.ParameterCodec)

  if executor, err = remotecommand.NewSPDYExecutor(restConf, "POST", sshReq.URL()); err != nil {
    goto END
  }

  handler = &streamHandler{
    wsConn:      wsConn,
    resizeEvent: make(chan remotecommand.TerminalSize),
  }
  if err = executor.Stream(remotecommand.StreamOptions{
    Stdin:             handler,
    Stdout:            handler,
    Stderr:            handler,
    TerminalSizeQueue: handler,
    Tty:               true,
  }); err != nil {
    goto END
  }
  return
END:
  fmt.Println(err)
  wsConn.WsClose()
}

func main() {
  var err error
  if clientset, err = common.InitClient(); err != nil {
    fmt.Println(err)
    return
  }
  http.HandleFunc("/ssh", wsHandler)
  err = http.ListenAndServe(":7888", nil)
  if err != nil {
    return
  }
}

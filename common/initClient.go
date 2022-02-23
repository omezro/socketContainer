package common

import (
	"io"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func InitClient() (clientset *kubernetes.Clientset, err error) {
  var restConf *rest.Config

  if restConf, err = GetRestConf(); err != nil {
    return
  }

  clientset, err = kubernetes.NewForConfig(restConf)
  return
}

func GetRestConf() (restConf *rest.Config, err error) {
  homeDir, err := os.UserHomeDir()
  if err != nil {
    return nil, err
  }

  file, err := os.Open(homeDir + "/.kube/config")
  if err != nil {
    restConf, err = rest.InClusterConfig()
  }else {
    defer file.Close()
    buf, err := io.ReadAll(file)
    if err != nil {
      return nil, err 
    }
    restConf, err = clientcmd.RESTConfigFromKubeConfig(buf)
  }
  return
}


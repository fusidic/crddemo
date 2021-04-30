package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/domac/crddemo/pkg/client/clientset/versioned"
	informers "github.com/domac/crddemo/pkg/client/informers/externalversions"
)

//程序启动参数
var (
	flagSet              = flag.NewFlagSet("crddemo", flag.ExitOnError)
	master               = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig           = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	onlyOneSignalHandler = make(chan struct{})
	shutdownSignals      = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

//设置信号处理
func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler)

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	// 确定需要捕获的信号
	signal.Notify(c, shutdownSignals...)
	go func() {
		// os.Interrupt
		<-c
		close(stop)
		// syscal.SIGTERM
		<-c
		os.Exit(1)
	}()

	return stop
}

func main() {
	flag.Parse()

	// 设置一个信号处理，用于优雅关闭，这个模式大多数程序都能这样写
	stopCh := setupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	mydemoClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %s", err.Error())
	}

	// informerFactory 工厂类，这里注入代码生成的 client
	// client 主要用于与 API Server 进行通信，实现 ListAndWatch
	mydemoInformerFactory := informers.NewSharedInformerFactory(mydemoClient, time.Second*30)

	// 生成一个 crddemo group 的 Mydemo 对象传递给自定义控制器
	controller := NewController(kubeClient, mydemoClient,
		mydemoInformerFactory.Crddemo().V1().Mydemos())

	go mydemoInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

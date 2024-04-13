package main

import (
	"github.com/alexflint/go-arg"
	"github.com/seculize/islazy/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/url"
	"os"
	"strings"
)

var InstallerArgs struct {
	Kubeconfig string `arg:"-k,--kubeconfig,required" help:"Path to the kubeconfig file to use for the Kubernetes cluster."`
	SMBURI     string `arg:"-s,--smb-uri,required" help:"SMB URI to the shared drive.\nExample: smb://<username>:<password>@<ip-address>/<share-name>"`
}

func setupLogs() {
	log.Output = "/dev/stdout"
	log.Level = log.INFO
	log.OnFatal = log.ExitOnFatal
	log.DateFormat = "06-Jan-02"
	log.TimeFormat = "15:04:05"
	log.DateTimeFormat = "2006-01-02 15:04:05"
	log.Format = "{datetime} {level:color}{level:name}{reset} {message}"
}
func main() {
	setupLogs()
	arg.MustParse(&InstallerArgs)
	// check if file exists
	if f, err := os.Stat(InstallerArgs.Kubeconfig); os.IsNotExist(err) || f.IsDir() {
		log.Fatal("Kubeconfig file does not exist or is a directory")
	}
	config, err := clientcmd.BuildConfigFromFlags("", InstallerArgs.Kubeconfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Info("Successfully connected to the Kubernetes cluster")
	// get kubernetes version
	version, err := clientset.ServerVersion()
	if err != nil {
		log.Fatal("Unable to detect Kubernetes version %s", err.Error())
	}
	log.Info("Kubernetes version: %s", version.Major+"."+version.Minor)
	getKubeVersion()
	startProcess("kubectl.exe", "apply", "-f", "configs/00-aws")
	startProcess("kubectl.exe", "apply", "-f", "configs/01-coturn")
	startProcess("kubectl.exe", "apply", "-f", "configs/02-csi-driver")
	uri, err := url.Parse(InstallerArgs.SMBURI)
	if err != nil {
		log.Fatal("Error parsing SMB URI: %s", err.Error())
	}
	validateURI(uri)
	pw, _ := uri.User.Password()
	startProcess("kubectl.exe", "delete", "secret", "smb-creds")
	startProcess("kubectl.exe", "create", "secret", "generic", "smb-creds", "--from-literal", "username="+uri.User.Username(), "--from-literal", "password="+pw)
	templateBytes, err := os.ReadFile("configs/03-csi-pv/01-create-pv.yaml")
	if err != nil {
		log.Fatal("Error reading file: %s", err.Error())
	}
	yamlString := string(templateBytes)
	yamlString = strings.ReplaceAll(yamlString, "%IP%", uri.Host)
	yamlString = strings.ReplaceAll(yamlString, "%PATH%", uri.Path)
	tmpFile, err := os.CreateTemp("", "01-create-pv.yaml")
	if err != nil {
		log.Fatal("Error creating temporary file: %s", err.Error())
	}
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString(yamlString)
	startProcess("kubectl.exe", "apply", "-f", tmpFile.Name())
	startProcess("kubectl.exe", "apply", "-f", "configs/04-gpu")

}

func validateURI(uri *url.URL) {
	if uri.Scheme != "smb" {
		log.Fatal("Invalid SMB URI: scheme must be 'smb'")
	}
	if uri.User == nil {
		log.Fatal("Invalid SMB URI: username is missing")
	}
	if uri.Host == "" {
		log.Fatal("Invalid SMB URI: host is missing")
	}
	if uri.Path == "" {
		log.Fatal("Invalid SMB URI: path is missing")
	}
	if uri.User.Username() == "" {
		log.Fatal("Invalid SMB URI: username is empty")
	}
	if _, err := uri.User.Password(); err != true {
		log.Fatal("Invalid SMB URI: password is empty")
	}
}

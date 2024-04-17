package main

import (
	"context"
	"github.com/alexflint/go-arg"
	"github.com/seculize/islazy/log"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/url"
	"os"
	"strings"
)

var InstallerArgs struct {
	Kubeconfig string `arg:"-k,--kubeconfig,required" help:"Path to the kubeconfig file to use for the Kubernetes cluster."`
	SMBURI     string `arg:"-s,--smb-uri,required" help:"SMB URI to the shared drive.\nExample: smb://<username>:<password>@<ip-address>/<share-name>"`
	CERT_PATH  string `arg:"-c,--cert-path,required" help:"Path to the certificate file to use for the Kubernetes cluster."`
	KEY_PATH   string `arg:"-p,--key-path,required" help:"Path to the key file to use for the Kubernetes cluster."`
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
	if f, err := os.Stat(InstallerArgs.CERT_PATH); os.IsNotExist(err) || f.IsDir() {
		log.Fatal("Certificate file does not exist or is a directory")
	}
	if f, err := os.Stat(InstallerArgs.KEY_PATH); os.IsNotExist(err) || f.IsDir() {
		log.Fatal("Key file does not exist or is a directory")
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
	// get aws-auth configmap
	patchRoles(clientset)
	startProcess("kubectl.exe", "apply", "-f", "configs/00-aws")
	startProcess("kubectl.exe", "apply", "-f", "configs/01-coturn")
	startProcess("kubectl.exe", "apply", "-f", "configs/02-csi-driver")
	uri, err := url.Parse(InstallerArgs.SMBURI)
	if err != nil {
		log.Fatal("Error parsing SMB URI: %s", err.Error())
	}
	validateURI(uri)
	pw, _ := uri.User.Password()
	startProcess("kubectl.exe", "delete", "secret", "smbcreds")
	startProcess("kubectl.exe", "create", "secret", "generic", "smbcreds", "--from-literal", "username="+uri.User.Username(), "--from-literal", "password="+pw)
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
	// create TLS secret
	startProcess("kubectl.exe", "delete", "-n", "games", "secret", "tls-secret")
	startProcess("kubectl.exe", "create", "-n", "games", "secret", "tls", "tls-secret", "--cert="+InstallerArgs.CERT_PATH, "--key="+InstallerArgs.KEY_PATH)
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
func patchRoles(clientset *kubernetes.Clientset) {
	configMap, err := clientset.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "aws-auth", metav1.GetOptions{})
	if err != nil {
		log.Error("Unable to get aws-auth configmap: %s", err.Error())
	} else {
		roles := configMap.Data["mapRoles"]
		// parse yaml and check if role exists
		var parsed []map[string]interface{}
		err := yaml.Unmarshal([]byte(roles), &parsed)
		if err != nil {
			log.Error("Error parsing aws-auth configmap: %s", err.Error())
		} else {
			var found = false
			clonedRoles := []map[string]interface{}{}
			for _, role := range parsed {
				if strings.Contains(role["rolearn"].(string), "WindowsWorker") {
					if len(role["groups"].([]interface{})) == 2 {
						found = true
						continue
					}
				}
				clonedRoles = append(clonedRoles, role)
			}
			if !found {
				log.Info("Roles already patched.")
				return
			}
			finalRoles, err := yaml.Marshal(clonedRoles)
			if err != nil {
				log.Error("Error marshalling yaml: %s", err.Error())
			} else {
				configMap.Data["mapRoles"] = string(finalRoles)
				_, err = clientset.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if err != nil {
					log.Error("Unable to update aws-auth configmap: %s", err.Error())
				} else {
					log.Info("Successfully updated aws-auth configmap")
				}
			}
		}
	}
}

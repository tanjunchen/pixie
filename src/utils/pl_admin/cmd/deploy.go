package cmd

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"pixielabs.ai/pixielabs/src/utils/pl_admin/cmd/k8s"
)

const (
	k8sMinVersion    = "1.8"
	kernelMinVersion = "4.14"
)

// NewCmdDeploy creates a new "deploy" command.
func NewCmdDeploy() *cobra.Command {
	return &cobra.Command{
		Use:   "deploy",
		Short: "Deploys Pixie on the current K8s cluster",
		Run: func(cmd *cobra.Command, args []string) {
			check, _ := cmd.Flags().GetBool("check")
			if check {
				checkCluster()
				return
			}

			kubeConfig := k8s.GetConfig()
			clientset := k8s.GetClientset(kubeConfig)
			namespace, _ := cmd.Flags().GetString("namespace")
			credsFile, _ := cmd.Flags().GetString("credentials_file")

			optionallyCreateNamespace(clientset, namespace)

			if credsFile != "" {
				secretName, _ := cmd.Flags().GetString("secret_name")
				k8s.CreateDockerConfigJSONSecret(clientset, namespace, secretName, credsFile)
			}

			path, _ := cmd.Flags().GetString("extract_yaml")
			extractYAMLs(path)

			versionString, err := cmd.Flags().GetString("use_version")
			if err != nil || len(versionString) == 0 {
				log.Fatal("Version string is invalid")
			}

			depsOnly, _ := cmd.Flags().GetBool("deps_only")

			err = updateYAMLsImageTag(path, versionString)
			if err != nil {
				log.WithError(err).Fatal("Failed to update image tags for YAML files.")
			}

			deploy(path, depsOnly)

			clusterID, _ := cmd.Flags().GetString("cluster_id")
			if clusterID == "" {
				log.Fatal("cluster_id is required")
			}
			jwtSigningKey := make([]byte, 64)
			_, err = rand.Read(jwtSigningKey)
			if err != nil {
				log.Fatal("Could not generate JWT signing key")
			}

			// Load clusterID and JWT signing key as a secret.
			k8s.DeleteSecret(clientset, namespace, "pl-cluster-secrets")
			k8s.CreateGenericSecretFromLiterals(clientset, namespace, "pl-cluster-secrets", map[string]string{
				"cluster-id":      clusterID,
				"jwt-signing-key": fmt.Sprintf("%x", jwtSigningKey),
			})
		},
	}
}

func optionallyCreateNamespace(clientset *kubernetes.Clientset, namespace string) {
	_, err := clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err == nil {
		return
	}
	_, err = clientset.CoreV1().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		log.WithError(err).Fatalf("Error creating namespace %s", namespace)
	}
	log.Infof("Created namespace %s", namespace)
}

func deploy(extractPath string, depsOnly bool) {
	// NATS and etcd deploys depend on timing, so may sometimes fail. Include some retry behavior.
	log.Info("Deploying NATS")
	retryDeploy(path.Join(extractPath, "nats.yaml"))
	log.Info("Deploying etcd")
	retryDeploy(path.Join(extractPath, "etcd.yaml"))

	if depsOnly {
		return
	}

	log.Info("Deploying Vizier")
	deployFile(path.Join(extractPath, "vizier.yaml"))
}

func deployFile(filePath string) error {
	kcmd := exec.Command("kubectl", "apply", "-f", filePath)
	return kcmd.Run()
}

func retryDeploy(filePath string) {
	tries := 5
	var err error
	for tries > 0 {
		err = deployFile(filePath)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
		tries--
	}
	if tries == 0 {
		log.WithError(err).Fatal(fmt.Sprintf("Could not deploy %s", filePath))
	}
}

// extractYAMLs extracts the yamls from the packaged bindata into the specified directory.
func extractYAMLs(extractPath string) {
	// Create directory for the yaml files.
	if _, err := os.Stat(extractPath); os.IsNotExist(err) {
		os.Mkdir(extractPath, 0777)
	}

	// Create a file for each asset, and write the asset's contents to the file.
	for _, asset := range AssetNames() {
		contents, err := Asset(asset)
		if err != nil {
			log.WithError(err).Fatal("Could not load asset")
		}
		fname := path.Join(extractPath, path.Base(asset))
		f, err := os.Create(fname)
		defer f.Close()
		err = ioutil.WriteFile(fname, contents, 0644)
		if err != nil {
			log.WithError(err).Fatal("Could not write to file")
		}
	}
}

func updateYAMLsImageTag(extractPath, versionString string) error {
	vizierYAMLPath := path.Join(extractPath, "vizier.yaml")
	c := exec.Command("sed", "-i", fmt.Sprintf(`s/\(image\:.*\:\)latest/\1%s/`, versionString),
		vizierYAMLPath)
	return c.Run()
}

// VersionCompatible checks whether a version is compatible, given a minimum version.
func VersionCompatible(version string, minVersion string) (bool, error) {
	versionParts := strings.Split(version, ".")
	if len(versionParts) != 2 {
		return false, errors.New("Version string incorrectly formatted")
	}
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return false, errors.New("Could not convert major version from string into int")
	}
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return false, errors.New("Could not convert minor version from string into int")
	}

	minVersionParts := strings.Split(minVersion, ".")
	if len(minVersionParts) != 2 {
		return false, errors.New("Min version string incorrectly formatted")
	}
	minMajor, err := strconv.Atoi(minVersionParts[0])
	if err != nil {
		return false, errors.New("Could not convert min major version from string into int")
	}
	minMinor, err := strconv.Atoi(minVersionParts[1])
	if err != nil {
		return false, errors.New("Could not convert min minor version from string into int")
	}

	if (major < minMajor) || (major == minMajor && minor < minMinor) {
		return false, nil
	}
	return true, nil
}

// checkCluster checks whether Pixie can properly run on the user's cluster.
func checkCluster() {
	kubeConfig := k8s.GetConfig()

	// Check K8s server version.
	discoveryClient := k8s.GetDiscoveryClient(kubeConfig)
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		log.WithError(err).Fatal("Could not get k8s server version")
	}
	log.Info(fmt.Sprintf("Kubernetes server version: %s.%s", version.Major, version.Minor))
	compatible, err := VersionCompatible(fmt.Sprintf("%s.%s", version.Major, version.Minor), k8sMinVersion)
	if err != nil {
		log.WithError(err).Fatal("Could not check k8s server version")
	}
	if !compatible {
		log.Info(fmt.Sprintf("Kubernetes server version not supported. Must have minimum version of %s", k8sMinVersion))
	}

	// Check version of each node.
	clientset := k8s.GetClientset(kubeConfig)
	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	log.Info(fmt.Sprintf("Checking kernel versions of nodes. Found %d node(s)", len(nodes.Items)))
	for _, node := range nodes.Items {
		compatible, err := VersionCompatible(node.Status.NodeInfo.KernelVersion, kernelMinVersion)
		if err != nil {
			log.WithError(err).Fatal("Could not check node kernel version")
		}
		if !compatible {
			log.Info(fmt.Sprintf("Kernel version for node %s not supported. Must have minimum kernel version of %s", node.Name, kernelMinVersion))
		}
	}
}

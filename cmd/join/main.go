package main

import (
	"log"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	tokenphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
)

const configMapName = "kurl-config"
const configMapNamespace = "kube-system"
const bootstrapTokenKey = "bootstrap_token"

func main() {
	client := clientsetOrDie()

	ti, err := GenerateBootstrapToken(client, nil)
	if err != nil {
		log.Panic(err)
	}

	// TODO kubeadm init phase upload-certs for HA

	// TODO rbac Get ConfigMap in kube-system namespace
	cm, err := client.CoreV1().ConfigMaps(configMapNamespace).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		log.Panic(err)
	}

	cm.Data[bootstrapTokenKey] = ti.Token

	_, err = client.CoreV1().ConfigMaps(configMapNamespace).Update(cm)
	if err != nil {
		log.Panic(err)
	}
}

type TokenInfo struct {
	Token    string
	CAHashes []string
}

// GenerateBootstrapToken will generate a node join token for kubeadm.
// ttl defines the time to live for this token. Defaults to 24h.
func GenerateBootstrapToken(client kubernetes.Interface, ttl *time.Duration) (*TokenInfo, error) {
	var tokenInfo TokenInfo

	token, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		return nil, errors.Wrap(err, "generate kubeadm token")
	}

	bts, err := kubeadm.NewBootstrapTokenString(token)
	if err != nil {
		return nil, errors.Wrap(err, "new kubeadm token string")
	}

	var duration *metav1.Duration
	if ttl != nil {
		duration = &metav1.Duration{Duration: *ttl}
	}

	// TODO rbac - Update, Create Secrets in kube-system namespace
	if err := tokenphase.UpdateOrCreateTokens(client, false, []kubeadm.BootstrapToken{
		{
			Token:  bts,
			TTL:    duration,
			Usages: []string{"authentication", "signing"},
			Groups: []string{kubeadmconstants.NodeBootstrapTokenAuthGroup},
		},
	}); err != nil {
		return nil, errors.Wrap(err, "create kubeadm token")
	}

	tokenInfo.Token = token

	// kurl-config should have the ca cert hash already
	// this function could just return a string

	/*
		rootCAFile := "/var/run/secrets/kubernetes.io/serviceaccount/" + v1.ServiceAccountRootCAKey
		caCerts, err := clientcertutil.CertsFromFile(rootCAFile)
		if err != nil {
			return nil, errors.Wrapf(err, "get certs from file %s", rootCAFile)
		}

		// hash all the CA certs and include their public key pins as trusted values
		tokenInfo.CAHashes = make([]string, 0, len(caCerts))
		for _, caCert := range caCerts {
			tokenInfo.CAHashes = append(tokenInfo.CAHashes, pubkeypin.Hash(caCert))
		}

	*/
	return &tokenInfo, nil
}

func clientsetOrDie() kubernetes.Interface {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic(err)
	}
	return clientset
}

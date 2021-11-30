package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/maksim-paskal/hcloud-node-health/pkg/config"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	nodeAnnotationPrefix           = "hcloud-node-health"
	hcloudProviderPrefix           = "hcloud://"
	nodeAnnotationLastReboot       = nodeAnnotationPrefix + "/last-reboot"
	nodeMinimumCreationMinutesDiff = 60
	nodeMinimumRebootMinutesDiff   = 10
)

var (
	ctx          = context.Background()
	clientset    *kubernetes.Clientset
	hcloudClient *hcloud.Client
)

func Init() error {
	var (
		err        error
		restconfig *rest.Config
	)

	if len(*config.Get().KubeConfigPath) > 0 {
		restconfig, err = clientcmd.BuildConfigFromFlags("", *config.Get().KubeConfigPath)
		if err != nil {
			return errors.Wrap(err, "error BuildConfigFromFlags")
		}
	} else {
		log.Info("No kubeconfig file use incluster")
		restconfig, err = rest.InClusterConfig()
		if err != nil {
			return errors.Wrap(err, "error InClusterConfig")
		}
	}

	clientset, err = kubernetes.NewForConfig(restconfig)
	if err != nil {
		return errors.Wrap(err, "error NewForConfig")
	}

	hcloudClient = hcloud.NewClient(hcloud.WithToken(*config.Get().HetznerToken))

	_, err = hcloudClient.Datacenter.All(ctx)
	if err != nil {
		return errors.Wrap(err, "error in hcloudClient")
	}

	return nil
}

func NodesCheck() error {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "error getting node list")
	}

	for _, node := range nodes.Items {
		if err := checkNode(node); err != nil {
			log.WithError(err).Error(node.Name)
		}
	}

	return nil
}

func checkNode(node corev1.Node) error { //nolint:funlen,cyclop
	log := log.WithFields(log.Fields{
		"node":        node.Name,
		"providerID":  node.Spec.ProviderID,
		"annotations": node.Annotations,
	})

	nodeReadyStatus := false

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			nodeReadyStatus = true
		}
	}

	log.Debugf("nodeReadyStatus=%t", nodeReadyStatus)

	if nodeReadyStatus {
		if err := clearNodeAnnotations(node); err != nil {
			return err
		}
		// if node in ready status - nothing to do
		return nil
	}

	if diffToNowMinutes(node.CreationTimestamp.Time) < nodeMinimumCreationMinutesDiff {
		log.Debugf("node lives less than %d minutes", nodeMinimumCreationMinutesDiff)

		return nil
	}

	hetznerServerID, err := getHetznerServer(node.Spec.ProviderID)
	if err != nil {
		return err
	}

	nodeAnnotationLastRebootValue := node.Annotations[nodeAnnotationLastReboot]

	if len(nodeAnnotationLastRebootValue) == 0 {
		log.Info("soft reboot")

		_, _, err := hcloudClient.Server.Reboot(ctx, hetznerServerID)
		if err != nil {
			return errors.Wrap(err, "error reboot server")
		}
	} else {
		lastRebootTime, err := time.Parse(time.RFC3339, nodeAnnotationLastRebootValue)
		if err != nil {
			return errors.Wrap(err, "error parsing annotation")
		}
		if diffToNowMinutes(lastRebootTime) > nodeMinimumRebootMinutesDiff {
			_, _, err := hcloudClient.Server.Reset(ctx, hetznerServerID)
			if err != nil {
				return errors.Wrap(err, "error reseting server")
			}
		}
	}

	err = saveNodeAnnotation(node.Name, nodeAnnotationLastReboot, time.Now().Format(time.RFC3339))
	if err != nil {
		return errors.Wrap(err, "error saving node annotation")
	}

	return nil
}

func clearNodeAnnotations(node corev1.Node) error {
	for k := range node.Annotations {
		if strings.HasPrefix(k, nodeAnnotationPrefix+"/") {
			if err := deleteNodeAnnotation(node.Name, k); err != nil {
				return err
			}
		}
	}

	return nil
}

func deleteNodeAnnotation(nodeName string, key string) error {
	keyFormatted := strings.ReplaceAll(key, "/", "~1")

	payload := fmt.Sprintf(`[{"op": "remove", "path": "/metadata/annotations/%s"}]`, keyFormatted)

	log.Debug(payload)

	nodes := clientset.CoreV1().Nodes()

	_, err := nodes.Patch(ctx, nodeName, types.JSONPatchType, []byte(payload), metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "error patching node")
	}

	return nil
}

func saveNodeAnnotation(nodeName string, key string, value string) error {
	type metadataStringValue struct {
		Annotations map[string]string `json:"annotations"`
	}

	type patchStringValue struct {
		Metadata metadataStringValue `json:"metadata"`
	}

	payload := patchStringValue{
		Metadata: metadataStringValue{
			Annotations: map[string]string{key: value},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "error marshaling payload")
	}

	nodes := clientset.CoreV1().Nodes()

	_, err = nodes.Patch(ctx, nodeName, types.StrategicMergePatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "error patching node")
	}

	return nil
}

func diffToNowMinutes(t time.Time) int {
	t1 := time.Now()

	return int(t1.Sub(t).Minutes())
}

func getHetznerServer(providerID string) (*hcloud.Server, error) {
	// get hetzner server
	hetznerServerID, err := strconv.Atoi(strings.TrimPrefix(providerID, hcloudProviderPrefix))
	if err != nil {
		return nil, errors.Wrap(err, "error formating providerID")
	}

	hcloudServer, _, err := hcloudClient.Server.GetByID(ctx, hetznerServerID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting server by id")
	}

	return hcloudServer, nil
}

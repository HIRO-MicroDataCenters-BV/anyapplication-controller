package errorctx

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kubernetes "k8s.io/client-go/kubernetes"
)

type LogFetcher interface {
	FetchLogs(ctx context.Context, namespace, podName, containerName string, previous bool) (string, error)
}

type RealLogFetcher struct {
	client kubernetes.Interface
}

func NewRealLogFetcher(client kubernetes.Interface) *RealLogFetcher {
	return &RealLogFetcher{client: client}
}

func (r *RealLogFetcher) FetchLogs(
	ctx context.Context, namespace, podName, containerName string,
	previous bool,
) (string, error) {
	req := r.client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		Previous:  previous,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var sb strings.Builder
	buf := make([]byte, 2048)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String(), nil
}

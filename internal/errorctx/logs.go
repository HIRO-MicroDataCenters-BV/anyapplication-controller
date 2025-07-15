package errorctx

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
)

type LogFetcher interface {
	FetchLogs(ctx context.Context, namespace, podName, containerName string, previous bool) (string, error)
	FetchEvents(ctx context.Context, namespace string) (*corev1.EventList, error)
}

type logFetcher struct {
	client kubernetes.Interface
}

func NewRealLogFetcher(client kubernetes.Interface) *logFetcher {
	return &logFetcher{client: client}
}

func (r *logFetcher) FetchEvents(
	ctx context.Context, namespace string,
) (*corev1.EventList, error) {
	events, err := r.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *logFetcher) FetchLogs(
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
	defer func() {
		err := stream.Close()
		if err != nil {
			fmt.Printf("Error closing log stream: %v\n", err)
		}
	}()

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

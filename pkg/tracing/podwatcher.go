package tracing

import (
	"context"
	"net/http"
	"strconv"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/pkg/logging"
)

const (
	// ServerPort specifies the port for http server
	ServerPort = 8080
)

// NewServer creates a new http server that exposes profiling data on the default profiling port
func NewServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    ":" + strconv.Itoa(ServerPort),
		Handler: handler,
	}
}

func WatchPods(ctx context.Context, namespace string) error {
	logger := logging.FromContext(ctx)
	client := kubeclient.Get(ctx)
	// lo := metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", name)}
	watch, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Errorw("Failed to set up pod watch for tracing", zap.Error(err))
		return err
	}
	defer watch.Stop()
	return TracePodStartup(ctx, watch.ResultChan())
}

// func ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	tryContext, trySpan := r.Context(), (*trace.Span)(nil)
// 	if tracingEnabled {
// 		tryContext, trySpan = trace.StartSpan(r.Context(), "throttler_try")
// 	}

// 	name := r.Header.Get(activator.RevisionHeaderName)
// 	namespace := r.Header.Get(activator.RevisionHeaderNamespace)
// 	if tracingConfig.KubeTracing && trySpan.SpanContext().IsSampled() {
// 		lo := metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", name)}
//
// 		if err != nil {
// 			logger.Errorw("Failed to set up pod watch for tracing", zap.Error(err))
// 		} else {
// 			defer watch.Stop()

// 			stopCh := make(chan struct{})
// 			defer close(stopCh)

// 			go tracing.TracePodStartup(tryContext, stopCh, watch.ResultChan())
// 		}
// 	}
// }

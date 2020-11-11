/*
Copyright 2019 The Knative Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tracing

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"knative.dev/pkg/logging"
)

const (
	csWaiting    = "Waiting"
	csRunning    = "Running"
	csReady      = "Ready"
	csTerminated = "Terminated"
)

type containerState string

type containerTrace struct {
	span  *trace.Span
	state containerState
}

type status struct {
	status    string
	timestamp time.Time
}

type ready struct {
	ready     bool
	timestamp time.Time
}

type podStatus struct {
	state          status
	containerState map[string]status
	containerReady map[string]ready
	createdTime    time.Time
}

// TracePodStartup creates spans detailing the startup process of the pod who's events arrive over eventCh
func TracePodStartup(ctx context.Context, eventCh <-chan watch.Event) error {
	logger := logging.FromContext(ctx)
	m := make(map[string]podStatus)

	for {
		select {
		case ev := <-eventCh:
			if pod, ok := ev.Object.(*corev1.Pod); ok {
				if pod.DeletionTimestamp != nil {
					delete(m, pod.Name)
					continue
				}
				if _, ok := m[pod.Name]; !ok {
					m[pod.Name] = podStatus{
						state: status{
							csWaiting,
							time.Now(),
						},
						containerState: make(map[string]status),
						containerReady: make(map[string]ready),
						createdTime:    time.Now(),
					}
				}
				prevStatus := m[pod.Name]
				// init containers
				for _, cs := range pod.Status.InitContainerStatuses {
					var currentStatus string
					if cs.State.Waiting != nil {
						currentStatus = csWaiting
					} else if cs.State.Running != nil {
						currentStatus = csRunning
					} else if cs.State.Terminated != nil {
						currentStatus = csTerminated
					}
					if prevStatus.containerState[cs.Name].status != currentStatus {
						prevStatus.containerState[cs.Name] = status{
							currentStatus, time.Now(),
						}
					}
					if cs.Ready && prevStatus.containerReady[cs.Name].ready != cs.Ready {
						prevStatus.containerReady[cs.Name] = ready{
							true, time.Now(),
						}
					}
				}

				for _, cs := range pod.Status.ContainerStatuses {
					var currentStatus string
					if cs.State.Waiting != nil {
						currentStatus = csWaiting
					} else if cs.State.Running != nil {
						currentStatus = csRunning
					} else if cs.State.Terminated != nil {
						currentStatus = csTerminated
					}
					if prevStatus.containerState[cs.Name].status != currentStatus {
						prevStatus.containerState[cs.Name] = status{
							currentStatus, time.Now(),
						}
					}
					if cs.Ready && prevStatus.containerReady[cs.Name].ready != cs.Ready {
						prevStatus.containerReady[cs.Name] = ready{
							true, time.Now(),
						}
					}
				}

				if pod.Status.Phase == corev1.PodRunning {
					if prevStatus.state.status == csWaiting {
						prevStatus.state.status = csRunning
						prevStatus.state.timestamp = time.Now()
					}
				}

				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						prevStatus.state.status = csReady
						prevStatus.state.timestamp = time.Now()
					}
				}
				m[pod.Name] = prevStatus
				var bs []byte
				writer := bytes.NewBuffer(bs)
				var delta time.Duration
				delta = prevStatus.state.timestamp.Sub(prevStatus.createdTime)
				fmt.Fprintf(writer, "\nPod: %s\n", pod.Name)
				fmt.Fprintf(writer, "  State: %10s, take %.2f seconds\n", prevStatus.state.status, delta.Seconds())
				fmt.Fprintf(writer, "  ContainerState:\n")

				var name []string

				for _, cs := range pod.Status.InitContainerStatuses {
					name = append(name, cs.Name)
				}

				for _, cs := range pod.Status.ContainerStatuses {
					name = append(name, cs.Name)
				}

				for _, n := range name {
					k := n
					v := prevStatus.containerState[n]
					delta = v.timestamp.Sub(prevStatus.createdTime)
					if prevStatus.containerReady[k].ready {
						delta = prevStatus.containerReady[k].timestamp.Sub(prevStatus.createdTime)
					}
					fmt.Fprintf(writer, "%20s: %-12s  Ready: %8t, take %.2f seconds\n", k, v.status, prevStatus.containerReady[k].ready, delta.Seconds())

				}
				fmt.Println(writer.String())
				logger.Info(writer.String())
			}
		case <-ctx.Done():
			return nil
		}
	}
}

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

package main

import (
	"log"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/knative-pod-tracing/pkg/tracing"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/version"
)

func main() {

	ctx := signals.NewContext()

	cfg := injection.ParseAndGetRESTConfigOrDie()

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	// Adjust our client's rate limits based on the number of controller's we are running.

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	kubeClient := kubeclient.Get(ctx)
	var err error
	var logger *zap.SugaredLogger

	baseLogger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to init zap logger %+v", err)
	}
	logger = baseLogger.Sugar()
	ctx = logging.WithLogger(ctx, logger)

	// We sometimes startup faster than we can reach kube-api. Poll on failure to prevent us terminating
	if perr := wait.PollImmediate(time.Second, 60*time.Second, func() (bool, error) {
		if err = version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
			log.Print("Failed to get k8s version ", err)
		}
		return err == nil, nil
	}); perr != nil {
		log.Fatal("Timed out attempting to get k8s version: ", err)
	}

	// Start all of the informers and wait for them to sync.
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatalw("Failed to start informers", zap.Error(err))
	}

	ns := "coligotest-1"
	err = tracing.WatchPods(ctx, ns)
	if err != nil {
		logger.Fatalw("watch pod exited", zap.Error(err))
	}
}

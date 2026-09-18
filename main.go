package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/steadybit/extension-auto-registration-ecs/autoregistration"
	extensionconfig "github.com/steadybit/extension-auto-registration-ecs/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extotel"
	"github.com/steadybit/extension-kit/extruntime"
	"log"
	"time"
)

func main() {
	extlogging.InitZeroLog()

	// Export OpenTelemetry traces when an OTLP endpoint is configured, so an
	// operator debugging a slow or timing-out action can see what happened inside
	// this extension. Off, and free, until OTEL_EXPORTER_OTLP_ENDPOINT is set —
	// see the extension-kit README for the full set of variables.
	extotel.InitOpenTelemetry()
	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.DebugLevel)
	extensionconfig.ParseConfiguration()

	awsCfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load AWS configuration: %v", err)
	}

	httpClientAgent := resty.New()
	httpClientAgent.BaseURL = "http://localhost:42899"
	httpClientAgent.SetDisableWarn(true)

	var ecsClient autoregistration.EcsApi = ecs.NewFromConfig(awsCfg)
	var ec2Client autoregistration.Ec2Api = ec2.NewFromConfig(awsCfg)

	for {
		//Sleep before first discovery to give the agent time to start
		time.Sleep(time.Duration(extensionconfig.Config.DiscoveryInterval) * time.Second)
		autoregistration.UpdateAgentExtensions(httpClientAgent, &ecsClient, &ec2Client)
	}
}

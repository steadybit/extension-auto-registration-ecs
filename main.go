package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	extensionconfig "github.com/steadybit/extension-discovery-ecs/config"
	"github.com/steadybit/extension-discovery-ecs/discovery"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	"log"
	"time"
)

func main() {
	extlogging.InitZeroLog()
	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.DebugLevel)
	extensionconfig.ParseConfiguration()

	awsCfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load AWS configuration: %v", err)
	}
	awsClient := ecs.NewFromConfig(awsCfg)

	client := resty.New()
	client.BaseURL = "http://localhost:42899"
	client.SetDisableWarn(true)

	for {
		//Sleep before first discovery to give the agent time to start
		time.Sleep(time.Duration(extensionconfig.Config.DiscoveryInterval) * time.Second)
		discovery.UpdateAgentExtensions(client, awsClient)
	}
}

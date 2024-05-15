package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// main uses the AWS SDK for Go V2 to create an Amazon Relational Database Service (Amazon RDS)
// client and list up to 20 DB instances in your account.
// This example uses the default settings specified in your shared credentials
// and config files.
func main() {
	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Println("Couldn't load default configuration. Have you set up your AWS account?")
		fmt.Println(err)
		return
	}
	accountClient := account.NewFromConfig(sdkConfig)
	regionOutput, err := accountClient.ListRegions(context.TODO(), &account.ListRegionsInput{
		RegionOptStatusContains: []types.RegionOptStatus{types.RegionOptStatusEnabled, types.RegionOptStatusEnabledByDefault}})
	if err != nil {
		fmt.Println("Couldn't list regions. Have you set your AWS account?")
	}
	//fmt.Println(regionOutput.Regions)
	for _, region := range regionOutput.Regions {
		const maxInstances = 20
		regionName := *region.RegionName

		rdsClient := rds.NewFromConfig(sdkConfig)
		//region := "eu-central-1"
		//fmt.Println("Let's list up to all DB instances.")
		var marker *string
		for {
			fmt.Printf("Let's list up to %v DB instances.\n", maxInstances)
			output, err := rdsClient.DescribeDBInstances(context.TODO(),
				&rds.DescribeDBInstancesInput{MaxRecords: aws.Int32(maxInstances), Marker: marker},
				func(o *rds.Options) { o.Region = regionName })
			if err != nil {
				fmt.Printf("Couldn't list DB instances: %v\n", err)
				return
			}
			//	fmt.Println(*output.Marker)
			if len(output.DBInstances) == 0 {
				fmt.Println("No DB instances found.")
			} else {
				for _, instance := range output.DBInstances {
					if instance.MaxAllocatedStorage != nil {
						fmt.Printf("RDS: %v az: %v alloc: %v maxalloc: %v.\n", *instance.DBInstanceIdentifier,
							*instance.AvailabilityZone, *instance.AllocatedStorage, *instance.MaxAllocatedStorage)
					} else {
						fmt.Printf("RDS: %v az: %v alloc: %v.\n", *instance.DBInstanceIdentifier,
							*instance.AvailabilityZone, *instance.AllocatedStorage)
					}
				}
			}
			if output.Marker == nil {
				break
			} else {
				fmt.Println(*output.Marker)
				marker = output.Marker
			}
		}
	}

	/*
		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			log.Fatalf("Couldn't marshal output to JSON: %v", err)
		}

		// Вывод структуры в формате JSON
		fmt.Println(string(jsonOutput))
	*/
}

package main

// format of relevant kernel specified options
// %e : executable path (without path prefix)
// %h : hostname
// %i : Thread id (in pid namespace)
// %I : Thread id (in initial namespace)
// %p : PID (pid namespace)
// %P : PID (in initial namespace)
// %s : signal number
// %t : time of dump in epoch seconds (utc)
// %u : uid of dumped process.

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// readerOnly is used to wrap os.Stdin so that it only
// exports the 'io.Reader' interface. Otherwise, the
// s3 uploader will try to Seek against it, causing
// an error.
type readerOnly struct {
	io.Reader
}

func main() {
	var bucket, region string
	flag.StringVar(&bucket, "bucket", "jonallie-s3core", "Bucket name")
	flag.StringVar(&region, "region", endpoints.UsEast2RegionID, "AWS Region")

	// Used for explicitly providing AWS crendentials.
	// Typically unused.
	var accessKey, secretKey, accessToken string
	flag.StringVar(&accessKey, "aws_access_key", "",
		"AWS access key (leave blank to use the environment or creds file)")
	flag.StringVar(&secretKey, "aws_secret_key", "",
		"AWS secret key (leave blank to use the environment or creds file)")
	flag.StringVar(&accessToken, "aws_access_token", "",
		"AWS access token (leave blank to use the environment or creds file)")

	// Used for specifying a shared credentials file.
	var credsFilename, credsProfile string
	flag.StringVar(&credsFilename, "creds_file", "", "Path the aws shared credentials file")
	flag.StringVar(&credsProfile, "creds_profile", "", "optional profile name for shared credentials")

	flag.Parse()

	if len(flag.Args()) < 4 {
		log.Fatal("expected <hostname> <exe> <pid> <time>")
	}

	sess := session.Must(session.NewSession(&aws.Config{
    CredentialsChainVerboseErrors: aws.Bool(true),
		Region: aws.String(region),
	}))

	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.StaticProvider{
				credentials.Value{
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					SessionToken:    accessToken,
				},
			},
			&credentials.SharedCredentialsProvider{
				Filename: credsFilename,
				Profile:  credsProfile,
			},
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(sess),
			},
		})

	sess.Config.Credentials = creds

	key := fmt.Sprintf("%s.%s.%s.%s.core", flag.Arg(0), flag.Arg(1), flag.Arg(2), flag.Arg(3))

	svc := s3.New(sess)
	uploader := s3manager.NewUploaderWithClient(svc)
	opts := &s3manager.UploadInput{
		ACL:    aws.String("bucket-owner-full-control"),
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   &readerOnly{os.Stdin},
	}
	result, err := uploader.Upload(opts)

	if err != nil {
		log.Fatal(err)
	}
	log.Println("Uploaded to ", result.Location)
}

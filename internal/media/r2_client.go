package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Client struct {
	client *s3.Client
	bucket string
}

func NewR2Client(accountID, accessKeyID, secretAccessKey, bucketName string) (*R2Client, error) {
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load R2 config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &R2Client{
		client: client,
		bucket: bucketName,
	}, nil
}

// GetObject načíta objekt z R2
func (r *R2Client) GetObject(ctx context.Context, key string) (io.ReadCloser, string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	output, err := r.client.GetObject(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object from R2: %w", err)
	}

	contentType := "image/jpeg" // default
	if output.ContentType != nil {
		contentType = *output.ContentType
	} else {
		// Detect content type from file extension if not provided
		if strings.HasSuffix(strings.ToLower(key), ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(strings.ToLower(key), ".webp") {
			contentType = "image/webp"
		} else if strings.HasSuffix(strings.ToLower(key), ".gif") {
			contentType = "image/gif"
		}
	}

	return output.Body, contentType, nil
}

// HeadObject získa metadata objektu bez sťahovania
func (r *R2Client) HeadObject(ctx context.Context, key string) (*s3.HeadObjectOutput, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	return r.client.HeadObject(ctx, input)
}

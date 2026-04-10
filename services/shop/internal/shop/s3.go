package shop

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Uploader uploads files to an S3-compatible store (MinIO).
type S3Uploader struct {
	client *s3.Client
	bucket string
}

// NewS3Uploader creates a new S3Uploader.
func NewS3Uploader(client *s3.Client, bucket string) *S3Uploader {
	return &S3Uploader{client: client, bucket: bucket}
}

// EnsureBucket creates the bucket if it does not already exist and sets a
// public-read policy so nginx can proxy GET requests without auth.
func (u *S3Uploader) EnsureBucket(ctx context.Context) error {
	_, err := u.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(u.bucket),
	})
	if err == nil {
		return nil
	}

	_, err = u.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(u.bucket),
	})
	if err != nil {
		return fmt.Errorf("create bucket %s: %w", u.bucket, err)
	}
	log.Printf("created S3 bucket: %s", u.bucket)

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, u.bucket)

	_, err = u.client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(u.bucket),
		Policy: aws.String(policy),
	})
	if err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}

	return nil
}

// Upload stores a file in S3.
func (u *S3Uploader) Upload(ctx context.Context, key string, contentType string, body io.Reader, size int64) error {
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(u.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

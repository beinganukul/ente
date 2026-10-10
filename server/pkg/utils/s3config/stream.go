package s3config

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// StreamObject copies with one buffered part instead of a temporary file.
func StreamObject(ctx context.Context, source *s3.S3, bucket *string, size int64, uploader *s3manager.Uploader, input *s3manager.UploadInput) (*s3manager.UploadOutput, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid object size: %d", size)
	}
	parts := uploader.MaxUploadParts
	if parts == 0 {
		parts = s3manager.MaxUploadParts
	}
	// shortcut: parts fit in Workers memory; use an external copier above this capacity.
	if parts < 1 || size > int64(parts)*16*1024*1024 {
		return nil, fmt.Errorf("object size %d exceeds the Worker streaming capacity for %d parts", size, parts)
	}
	partSize := max(s3manager.MinUploadPartSize, (size+int64(parts)-1)/int64(parts))
	object, err := source.GetObjectWithContext(ctx, &s3.GetObjectInput{Bucket: bucket, Key: input.Key})
	if err != nil {
		return nil, err
	}
	defer object.Body.Close()
	if aws.Int64Value(object.ContentLength) != size {
		return nil, fmt.Errorf("source object size does not match expected size %d", size)
	}
	input.Body = &sizedReader{reader: io.LimitReader(object.Body, size+1), expected: size}
	return uploader.UploadWithContext(ctx, input, func(u *s3manager.Uploader) { u.Concurrency = 1; u.PartSize = partSize })
}

type sizedReader struct {
	reader         io.Reader
	expected, read int64
}

func (r *sizedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if r.read > r.expected || (err == io.EOF && r.read != r.expected) {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

func (c *S3Config) StreamCopy(ctx context.Context, sourceID, destinationID, key string, size int64) error {
	source, destination := c.GetS3Client(sourceID), c.GetS3Client(destinationID)
	_, err := StreamObject(ctx, &source, c.GetBucket(sourceID), size, s3manager.NewUploaderWithClient(&destination), &s3manager.UploadInput{Bucket: c.GetBucket(destinationID), Key: aws.String(key)})
	if err != nil {
		return err
	}
	object, err := destination.HeadObjectWithContext(ctx, &s3.HeadObjectInput{Bucket: c.GetBucket(destinationID), Key: aws.String(key)})
	if err != nil {
		return err
	}
	if aws.Int64Value(object.ContentLength) != size {
		return fmt.Errorf("destination object size does not match expected size %d", size)
	}
	return nil
}

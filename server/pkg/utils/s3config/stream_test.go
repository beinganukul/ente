package s3config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type s3Transport func(*http.Request) (*http.Response, error)

func (f s3Transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStreamObject(t *testing.T) {
	for _, length := range []int{21, 6 * 1024 * 1024} {
		payload := bytes.Repeat([]byte{'x'}, length)
		for _, mismatch := range []bool{false, true} {
			var uploaded []byte
			config := &aws.Config{Endpoint: aws.String("https://s3.example.com"), Region: aws.String("test"), Credentials: credentials.NewStaticCredentials("test", "test", ""), S3ForcePathStyle: aws.Bool(true), HTTPClient: &http.Client{Transport: s3Transport(func(r *http.Request) (*http.Response, error) {
				response := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}
				switch {
				case r.Method == http.MethodGet:
					response.Body = io.NopCloser(bytes.NewReader(payload))
					response.ContentLength = int64(len(payload))
					response.Header.Set("Content-Length", fmt.Sprint(len(payload)))
				case r.Method == http.MethodPost && r.URL.Query().Has("uploads"):
					response.Body = io.NopCloser(strings.NewReader("<InitiateMultipartUploadResult><Bucket>target</Bucket><Key>key</Key><UploadId>test</UploadId></InitiateMultipartUploadResult>"))
				case r.Method == http.MethodPut:
					data, err := io.ReadAll(r.Body)
					if err != nil {
						return nil, err
					}
					uploaded = append(uploaded, data...)
					response.Header.Set("ETag", "test")
				case r.Method == http.MethodPost && r.URL.Query().Has("uploadId"):
					response.Body = io.NopCloser(strings.NewReader("<CompleteMultipartUploadResult><Location>https://s3.example.com/target/key</Location><Bucket>target</Bucket><Key>key</Key><ETag>test</ETag></CompleteMultipartUploadResult>"))
				default:
					return nil, fmt.Errorf("unexpected S3 request %s %s", r.Method, r.URL)
				}
				return response, nil
			})}}
			client := s3.New(session.Must(session.NewSession(config)))
			size := int64(len(payload))
			if mismatch {
				size++
			}
			_, err := StreamObject(context.Background(), client, aws.String("source"), size, s3manager.NewUploaderWithClient(client), &s3manager.UploadInput{Bucket: aws.String("target"), Key: aws.String("key")})
			if mismatch {
				if err == nil {
					t.Fatal("accepted incorrect source size")
				}
				if uploaded != nil {
					t.Fatal("uploaded incorrectly sized source")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(uploaded, payload) {
					t.Fatalf("copy mismatch: copied %d, expected %d", len(uploaded), len(payload))
				}
			}
		}
	}
}
func TestSizedReaderRejectsTruncatedAndOversizedData(t *testing.T) {
	for _, input := range []string{"short", "too much data"} {
		_, err := io.ReadAll(&sizedReader{reader: bytes.NewBufferString(input), expected: 8})
		if err != io.ErrUnexpectedEOF {
			t.Fatalf("input %q: %v", input, err)
		}
	}
}

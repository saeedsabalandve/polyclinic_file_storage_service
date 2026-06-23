package repository

import (
    "context"
    "crypto/md5"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "go.uber.org/zap"

    appConfig "github.com/polyclinic/file-storage-service/internal/config"
)

// S3Client wraps the AWS S3 client
type S3Client struct {
    client     *s3.Client
    config     *appConfig.Config
    logger     *zap.Logger
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *appConfig.Config) (*S3Client, error) {
    var awsConfig aws.Config
    var err error

    if cfg.AWSAccessKeyID != "" && cfg.AWSSecretAccessKey != "" {
        // Use static credentials
        awsConfig, err = config.LoadDefaultConfig(context.TODO(),
            config.WithRegion(cfg.AWSRegion),
            config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
                cfg.AWSAccessKeyID,
                cfg.AWSSecretAccessKey,
                "",
            )),
        )
    } else {
        // Use default credential chain (IAM roles, etc.)
        awsConfig, err = config.LoadDefaultConfig(context.TODO(),
            config.WithRegion(cfg.AWSRegion),
        )
    }

    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    // Configure S3 client
    client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
        if cfg.S3Endpoint != "" {
            o.BaseEndpoint = aws.String(cfg.S3Endpoint)
            o.UsePathStyle = cfg.S3UsePathStyle
        }
    })

    logger, _ := zap.NewProduction()

    return &S3Client{
        client: client,
        config: cfg,
        logger: logger,
    }, nil
}

// CreateTenantBucket creates a new S3 bucket for a tenant
func (s *S3Client) CreateTenantBucket(ctx context.Context, tenantID string) error {
    bucketName := s.getTenantBucketName(tenantID)

    // Create bucket
    _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
        Bucket: aws.String(bucketName),
        CreateBucketConfiguration: &types.CreateBucketConfiguration{
            LocationConstraint: types.BucketLocationConstraint(s.config.AWSRegion),
        },
    })
    if err != nil {
        return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
    }

    // Enable versioning
    if s.config.EnableVersioning {
        _, err = s.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
            Bucket: aws.String(bucketName),
            VersioningConfiguration: &types.VersioningConfiguration{
                Status: types.BucketVersioningStatusEnabled,
            },
        })
        if err != nil {
            return fmt.Errorf("failed to enable versioning: %w", err)
        }
    }

    // Enable encryption
    if s.config.EnableEncryption {
        _, err = s.client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
            Bucket: aws.String(bucketName),
            ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
                Rules: []types.ServerSideEncryptionRule{
                    {
                        ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
                            SSEAlgorithm: types.ServerSideEncryptionAes256,
                        },
                    },
                },
            },
        })
        if err != nil {
            return fmt.Errorf("failed to enable encryption: %w", err)
        }
    }

    // Set lifecycle rules
    if s.config.LifecycleRules.EnableLifecycle {
        err = s.setLifecycleRules(ctx, bucketName)
        if err != nil {
            return fmt.Errorf("failed to set lifecycle rules: %w", err)
        }
    }

    // Block public access
    _, err = s.client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
        Bucket: aws.String(bucketName),
        PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
            BlockPublicAcls:       aws.Bool(true),
            BlockPublicPolicy:     aws.Bool(true),
            IgnorePublicAcls:      aws.Bool(true),
            RestrictPublicBuckets: aws.Bool(true),
        },
    })
    if err != nil {
        return fmt.Errorf("failed to block public access: %w", err)
    }

    s.logger.Info("Created tenant bucket", 
        zap.String("bucket", bucketName),
        zap.String("tenant_id", tenantID))

    return nil
}

// UploadFile uploads a file to S3
func (s *S3Client) UploadFile(ctx context.Context, tenantID, objectKey string, reader io.Reader, contentType string, metadata map[string]string) (*s3.PutObjectOutput, error) {
    bucketName := s.getTenantBucketName(tenantID)

    input := &s3.PutObjectInput{
        Bucket:      aws.String(bucketName),
        Key:         aws.String(objectKey),
        Body:        reader,
        ContentType: aws.String(contentType),
        Metadata:    metadata,
    }

    // Add server-side encryption
    if s.config.EnableEncryption {
        if s.config.EncryptionType == "SSE-KMS" && s.config.KMSKeyID != "" {
            input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
            input.SSEKMSKeyId = aws.String(s.config.KMSKeyID)
        } else {
            input.ServerSideEncryption = types.ServerSideEncryptionAes256
        }
    }

    output, err := s.client.PutObject(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("failed to upload file: %w", err)
    }

    return output, nil
}

// DownloadFile downloads a file from S3
func (s *S3Client) DownloadFile(ctx context.Context, tenantID, objectKey string) (*s3.GetObjectOutput, error) {
    bucketName := s.getTenantBucketName(tenantID)

    output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(objectKey),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to download file: %w", err)
    }

    return output, nil
}

// DeleteFile deletes a file from S3
func (s *S3Client) DeleteFile(ctx context.Context, tenantID, objectKey string) error {
    bucketName := s.getTenantBucketName(tenantID)

    _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(objectKey),
    })
    if err != nil {
        return fmt.Errorf("failed to delete file: %w", err)
    }

    return nil
}

// GeneratePresignedURL generates a pre-signed URL for download/upload
func (s *S3Client) GeneratePresignedURL(ctx context.Context, tenantID, objectKey string, expiration time.Duration, method string) (string, error) {
    bucketName := s.getTenantBucketName(tenantID)

    var presignedURL string
    var err error

    switch method {
    case "GET":
        presignedClient := s3.NewPresignClient(s.client)
        req, err := presignedClient.PresignGetObject(ctx, &s3.GetObjectInput{
            Bucket: aws.String(bucketName),
            Key:    aws.String(objectKey),
        }, s3.WithPresignExpires(expiration))
        if err != nil {
            return "", fmt.Errorf("failed to generate presigned URL: %w", err)
        }
        presignedURL = req.URL
    case "PUT":
        presignedClient := s3.NewPresignClient(s.client)
        req, err := presignedClient.PresignPutObject(ctx, &s3.PutObjectInput{
            Bucket: aws.String(bucketName),
            Key:    aws.String(objectKey),
        }, s3.WithPresignExpires(expiration))
        if err != nil {
            return "", fmt.Errorf("failed to generate presigned URL: %w", err)
        }
        presignedURL = req.URL
    default:
        return "", fmt.Errorf("unsupported HTTP method: %s", method)
    }

    return presignedURL, nil
}

// InitiateMultipartUpload initiates a multipart upload
func (s *S3Client) InitiateMultipartUpload(ctx context.Context, tenantID, objectKey, contentType string) (*s3.CreateMultipartUploadOutput, error) {
    bucketName := s.getTenantBucketName(tenantID)

    input := &s3.CreateMultipartUploadInput{
        Bucket:      aws.String(bucketName),
        Key:         aws.String(objectKey),
        ContentType: aws.String(contentType),
    }

    if s.config.EnableEncryption {
        input.ServerSideEncryption = types.ServerSideEncryptionAes256
    }

    output, err := s.client.CreateMultipartUpload(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("failed to initiate multipart upload: %w", err)
    }

    return output, nil
}

// UploadPart uploads a part of a multipart upload
func (s *S3Client) UploadPart(ctx context.Context, tenantID, objectKey, uploadID string, partNumber int32, reader io.ReadSeeker, size int64) (*s3.UploadPartOutput, error) {
    bucketName := s.getTenantBucketName(tenantID)

    output, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
        Bucket:     aws.String(bucketName),
        Key:        aws.String(objectKey),
        UploadId:   aws.String(uploadID),
        PartNumber: aws.Int32(partNumber),
        Body:       reader,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to upload part %d: %w", partNumber, err)
    }

    return output, nil
}

// CompleteMultipartUpload completes a multipart upload
func (s *S3Client) CompleteMultipartUpload(ctx context.Context, tenantID, objectKey, uploadID string, parts []types.CompletedPart) error {
    bucketName := s.getTenantBucketName(tenantID)

    _, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
        Bucket:   aws.String(bucketName),
        Key:      aws.String(objectKey),
        UploadId: aws.String(uploadID),
        MultipartUpload: &types.CompletedMultipartUpload{
            Parts: parts,
        },
    })
    if err != nil {
        return fmt.Errorf("failed to complete multipart upload: %w", err)
    }

    return nil
}

// AbortMultipartUpload aborts a multipart upload
func (s *S3Client) AbortMultipartUpload(ctx context.Context, tenantID, objectKey, uploadID string) error {
    bucketName := s.getTenantBucketName(tenantID)

    _, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
        Bucket:   aws.String(bucketName),
        Key:      aws.String(objectKey),
        UploadId: aws.String(uploadID),
    })
    if err != nil {
        return fmt.Errorf("failed to abort multipart upload: %w", err)
    }

    return nil
}

// GetObjectMetadata returns object metadata
func (s *S3Client) GetObjectMetadata(ctx context.Context, tenantID, objectKey string) (*s3.HeadObjectOutput, error) {
    bucketName := s.getTenantBucketName(tenantID)

    output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(objectKey),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get object metadata: %w", err)
    }

    return output, nil
}

// CalculateFileHash calculates MD5 and SHA256 hashes
func CalculateFileHash(reader io.Reader) (string, string, error) {
    md5Hash := md5.New()
    sha256Hash := sha256.New()

    teeReader := io.TeeReader(reader, md5Hash)
    teeReader = io.TeeReader(teeReader, sha256Hash)

    if _, err := io.Copy(io.Discard, teeReader); err != nil {
        return "", "", err
    }

    return hex.EncodeToString(md5Hash.Sum(nil)), hex.EncodeToString(sha256Hash.Sum(nil)), nil
}

// Helper functions
func (s *S3Client) getTenantBucketName(tenantID string) string {
    return fmt.Sprintf("%s-%s-storage", s.config.S3BucketPrefix, tenantID)
}

func (s *S3Client) setLifecycleRules(ctx context.Context, bucketName string) error {
    lifecycleConfig := &types.BucketLifecycleConfiguration{
        Rules: []types.LifecycleRule{
            {
                ID:     aws.String("transition-to-ia"),
                Status: types.ExpirationStatusEnabled,
                Transitions: []types.Transition{
                    {
                        Days:         aws.Int32(int32(s.config.LifecycleRules.TransitionToIA)),
                        StorageClass: types.TransitionStorageClassStandardIa,
                    },
                },
                Filter: &types.LifecycleRuleFilter{
                    Prefix: aws.String(""),
                },
            },
            {
                ID:     aws.String("transition-to-glacier"),
                Status: types.ExpirationStatusEnabled,
                Transitions: []types.Transition{
                    {
                        Days:         aws.Int32(int32(s.config.LifecycleRules.TransitionToGlacier)),
                        StorageClass: types.TransitionStorageClassGlacier,
                    },
                },
                Filter: &types.LifecycleRuleFilter{
                    Prefix: aws.String("archived/"),
                },
            },
            {
                ID:     aws.String("expiration"),
                Status: types.ExpirationStatusEnabled,
                Expiration: &types.LifecycleExpiration{
                    Days: aws.Int32(int32(s.config.LifecycleRules.ExpirationDays)),
                },
                Filter: &types.LifecycleRuleFilter{
                    Prefix: aws.String(""),
                },
            },
        },
    }

    _, err := s.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
        Bucket:                 aws.String(bucketName),
        LifecycleConfiguration: lifecycleConfig,
    })

    return err
}

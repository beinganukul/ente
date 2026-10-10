package ente

import (
	"database/sql"
	"testing"
)

func TestJSONScannersAcceptTextAndBytes(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func() sql.Scanner
	}{
		{"collection", func() sql.Scanner { return &CollectionAttributes{} }},
		{"subscription", func() sql.Scanner { return &SubscriptionAttributes{} }},
		{"file", func() sql.Scanner { return &FileInfo{} }},
		{"metadata", func() sql.Scanner { return &MagicMetadata{} }},
		{"abuse report", func() sql.Scanner { return &AbuseReportDetails{} }},
		{"reporter", func() sql.Scanner { return &ReporterAddress{} }},
	} {
		t.Run(factory.name, func(t *testing.T) {
			for _, input := range []any{`{"version":2}`, []byte(`{"version":2}`)} {
				if err := factory.new().Scan(input); err != nil {
					t.Fatalf("scan %T: %v", input, err)
				}
			}
			if err := factory.new().Scan(42); err == nil {
				t.Fatal("numeric JSON database value must be rejected")
			}
			if err := factory.new().Scan("invalid JSON"); err == nil {
				t.Fatal("malformed JSON must be rejected")
			}
		})
	}
}

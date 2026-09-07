package storageclassmap

import "testing"

func TestStorageGroups(t *testing.T) {
	groups := KnownStorageGroups()
	if len(groups) != 8 {
		t.Fatalf("expected 8 storage groups, got %d", len(groups))
	}

	tests := []struct {
		raw      string
		expected string
	}{
		{"Amazon S3 Glacier Flexible", StorageGroupArchive},
		{"Azure Blob Archive", StorageGroupArchive},
		{"EBS gp3", StorageGroupBlock},
		{"Azure Managed Disks Premium SSD", StorageGroupBlock},
		{"Google Cloud Persistent Disk", StorageGroupBlock},
		{"Amazon EFS", StorageGroupFile},
		{"Azure Files Hot", StorageGroupFile},
		{"Google Cloud Filestore", StorageGroupFile},
		{"AWS Backup Vault", StorageGroupBackupDR},
		{"Azure Site Recovery", StorageGroupBackupDR},
		{"AWS Storage Gateway", StorageGroupHybrid},
		{"Azure Data Box Heavy", StorageGroupHybrid},
		{"AWS DataSync", StorageGroupMigration},
		{"Azure Storage Mover", StorageGroupMigration},
		{"Google Cloud Storage Transfer Service", StorageGroupMigration},
		{"Amazon FSx for Lustre", StorageGroupSpecializedHPC},
		{"Google Cloud Parallelstore", StorageGroupSpecializedHPC},
		{"Amazon S3 Standard", StorageGroupObject},
		{"Azure Blob Hot", StorageGroupObject},
		{"Google Cloud Storage Standard", StorageGroupObject},
	}

	for _, tt := range tests {
		got := MapStorageGroup(tt.raw)
		if got != tt.expected {
			t.Errorf("MapStorageGroup(%q) = %q, want %q", tt.raw, got, tt.expected)
		}
	}
}

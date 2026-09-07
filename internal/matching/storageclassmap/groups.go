package storageclassmap

import "strings"

// Canonical storage groups across cloud providers.
const (
	StorageGroupObject         = "object"
	StorageGroupBlock          = "block"
	StorageGroupFile           = "file"
	StorageGroupArchive        = "archive"
	StorageGroupBackupDR       = "backup_dr"
	StorageGroupHybrid         = "hybrid"
	StorageGroupMigration      = "migration"
	StorageGroupSpecializedHPC = "specialized_hpc"
)

// KnownStorageGroups returns all 8 canonical storage groups.
func KnownStorageGroups() []string {
	return []string{
		StorageGroupObject,
		StorageGroupBlock,
		StorageGroupFile,
		StorageGroupArchive,
		StorageGroupBackupDR,
		StorageGroupHybrid,
		StorageGroupMigration,
		StorageGroupSpecializedHPC,
	}
}

// MapStorageGroup resolves a raw product or storage class string to a canonical storage group.
func MapStorageGroup(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "glacier") || strings.Contains(s, "archive") || strings.Contains(s, "coldline"):
		return StorageGroupArchive
	case strings.Contains(s, "ebs") || strings.Contains(s, "disk") || strings.Contains(s, "hyperdisk"):
		return StorageGroupBlock
	case strings.Contains(s, "efs") || strings.Contains(s, "fsx") && !strings.Contains(s, "lustre") ||
		strings.Contains(s, "azure files") || strings.Contains(s, "netapp") || strings.Contains(s, "filestore"):
		return StorageGroupFile
	case strings.Contains(s, "backup") || strings.Contains(s, "disaster recovery") || strings.Contains(s, "site recovery"):
		return StorageGroupBackupDR
	case strings.Contains(s, "gateway") || strings.Contains(s, "snowball") || strings.Contains(s, "data box") || strings.Contains(s, "storsimple"):
		return StorageGroupHybrid
	case strings.Contains(s, "datasync") || strings.Contains(s, "storage mover") || strings.Contains(s, "transfer service") || strings.Contains(s, "transfer appliance"):
		return StorageGroupMigration
	case strings.Contains(s, "lustre") || strings.Contains(s, "hpc cache") || strings.Contains(s, "parallelstore"):
		return StorageGroupSpecializedHPC
	default:
		return StorageGroupObject
	}
}

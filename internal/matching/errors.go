package matching

import (
	"errors"

	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/databaseenginemap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

// IsUnmappedError returns true if the error (or any error in its chain) is one of
// the curated taxonomy sentinel errors indicating an unmapped product, region,
// storage class, transfer type, or database engine.
func IsUnmappedError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, catalogmap.ErrUnmappedProduct) ||
		errors.Is(err, regionmap.ErrUnmappedRegion) ||
		errors.Is(err, storageclassmap.ErrUnmappedStorageClass) ||
		errors.Is(err, transfertypemap.ErrUnmappedTransferType) ||
		errors.Is(err, databaseenginemap.ErrUnmappedDatabaseEngine)
}

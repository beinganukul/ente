package repo

import (
	"testing"

	"github.com/ente/museum/ente"
	"github.com/ente/museum/pkg/repo/public"
	"github.com/stretchr/testify/require"
)

func TestSpecialCollectionDuplicateErrorsAcrossProviders(t *testing.T) {
	repository, db, ownerID := setupCollectionMembershipTest(t)
	for _, tc := range []struct {
		kind string
		err  error
	}{
		{"favorites", ente.ErrFavoriteCollectionAlreadyExist},
		{"uncategorized", ente.ErrUncategorizeCollectionAlreadyExists},
	} {
		collection := ente.Collection{Owner: ente.CollectionUser{ID: ownerID}, Type: tc.kind, App: string(ente.Photos), EncryptedKey: "key", KeyDecryptionNonce: "nonce"}
		created, err := repository.Create(collection)
		require.NoError(t, err)
		_, err = repository.Create(collection)
		require.ErrorIs(t, err, tc.err)
		collection.App = string(ente.Locker)
		_, err = repository.Create(collection)
		require.NoError(t, err, "special collections are unique per app")

		links := public.CollectionLinkRepo{DB: db}
		require.NoError(t, links.Insert(t.Context(), created.ID, tc.kind+"-first", 0, 0, false, false, nil))
		err = links.Insert(t.Context(), created.ID, tc.kind+"-second", 0, 0, false, false, nil)
		require.ErrorIs(t, err, ente.ErrActiveLinkAlreadyExists)
	}
}

package repo

import (
	"bytes"
	"database/sql"
	"testing"

	"github.com/ente/museum/ente"
	"github.com/stretchr/testify/require"
)

func TestUpdateSlugPreservesKeysAndClassifiesConflicts(t *testing.T) {
	module := newSpaceTestModule(t)
	alice, bob := notificationTestSpaces(t, module)
	updated, err := module.Spaces.UpdateSlug(t.Context(), alice.SpaceID, "new_alice_slug")
	require.NoError(t, err)
	require.Equal(t, "new_alice_slug", updated.SpaceSlug)
	require.Equal(t, alice.RootWrappedSpaceKey, updated.RootWrappedSpaceKey)
	require.Equal(t, alice.CurrentVersion, updated.CurrentVersion)
	_, err = module.Spaces.UpdateSlug(t.Context(), alice.SpaceID, bob.SpaceSlug)
	require.Error(t, err)
	require.IsType(t, ente.NewAlreadyExistsError(""), err)
	_, err = module.Spaces.UpdateSlug(t.Context(), "missing-space", "unused_slug")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestMessageReactionPreservesFirstLikeTimeAndChecksRecipient(t *testing.T) {
	module := newSpaceTestModule(t)
	alice, bob := notificationTestSpaces(t, module)
	message, err := module.Messages.CreateMessage(t.Context(), CreateSpaceMessageRecord{
		SenderSpaceID: alice.SpaceID, RecipientSpaceID: bob.SpaceID, Kind: "regular",
		MessageCipher: []byte("cipher"), SenderEncryptedMessageKey: []byte("sender-key"), RecipientEncryptedMessageKey: []byte("recipient-key"),
	})
	require.NoError(t, err)
	_, err = module.Messages.SetReaction(t.Context(), message.MessageID, alice.SpaceID, bytes.Repeat([]byte("s"), 304), bytes.Repeat([]byte("r"), 304))
	require.ErrorIs(t, err, sql.ErrNoRows)
	added, err := module.Messages.SetReaction(t.Context(), message.MessageID, bob.SpaceID, bytes.Repeat([]byte("s"), 304), bytes.Repeat([]byte("r"), 304))
	require.NoError(t, err)
	require.True(t, added)
	var firstLikedAt int64
	require.NoError(t, module.Messages.DB.QueryRow(`SELECT recipient_liked_at FROM space_messages WHERE message_id = $1`, message.MessageID).Scan(&firstLikedAt))
	added, err = module.Messages.SetReaction(t.Context(), message.MessageID, bob.SpaceID, bytes.Repeat([]byte("s2"), 152), bytes.Repeat([]byte("r2"), 152))
	require.NoError(t, err)
	require.False(t, added)
	var latestLikedAt int64
	require.NoError(t, module.Messages.DB.QueryRow(`SELECT recipient_liked_at FROM space_messages WHERE message_id = $1`, message.MessageID).Scan(&latestLikedAt))
	require.Equal(t, firstLikedAt, latestLikedAt)
	recipient, err := module.Messages.GetMessage(t.Context(), message.MessageID, bob.SpaceID)
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte("r2"), 152), recipient.EncryptedReaction)
	require.NoError(t, module.Messages.DeleteMessage(t.Context(), message.MessageID, alice.SpaceID))
	_, err = module.Messages.SetReaction(t.Context(), message.MessageID, bob.SpaceID, bytes.Repeat([]byte("s3"), 152), bytes.Repeat([]byte("r3"), 152))
	require.ErrorIs(t, err, sql.ErrNoRows)
}

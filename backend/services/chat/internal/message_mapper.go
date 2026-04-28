package chat

func MessageResponseFromMessageDB(messageDB *MessageDB) *MessageResponse {
	return &MessageResponse{
		Id:           messageDB.Id,
		SenderID:     messageDB.SenderID,
		ReceiverID:   messageDB.ReceiverID,
		Content:      messageDB.Content,
		Attachments:  normalizeAttachments(messageDB.Attachments),
		Timestamp:    messageDB.Timestamp,
		ViewedStatus: NormalizeViewedStatus(messageDB.ViewedStatus),
	}
}

func normalizeAttachments(attachments []AttachmentSnapshot) []AttachmentSnapshot {
	if attachments == nil {
		return []AttachmentSnapshot{}
	}
	return attachments
}

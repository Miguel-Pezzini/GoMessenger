package chat

func MessageResponseFromMessageDB(messageDB *MessageDB) *MessageResponse {
	return &MessageResponse{
		Id:         messageDB.Id,
		SenderID:   messageDB.SenderID,
		ReceiverID: messageDB.ReceiverID,
		Content:    messageDB.Content,
		Timestamp:  messageDB.Timestamp,
	}
}

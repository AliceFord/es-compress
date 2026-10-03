package esc

import (
	"io"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func (p *Parser) parseMessageRecord(r io.Reader) (record.MessageRecord, error) {
	senderID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.MessageRecord{}, err
	}

	sender, ok := p.textMap[uint16(senderID)]
	if !ok {
		return record.MessageRecord{}, err
	}

	receiverID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.MessageRecord{}, err
	}

	receiver, ok := p.textMap[uint16(receiverID)]
	if !ok {
		return record.MessageRecord{}, err
	}

	message, err := binio.ReadCString(r)
	if err != nil {
		return record.MessageRecord{}, err
	}

	return record.MessageRecord{
		Sender:   sender,
		Receiver: receiver,
		Message:  message,
	}, nil
}

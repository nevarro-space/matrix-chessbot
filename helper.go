package main

import (
	"errors"
	"fmt"
	_ "strconv"
	"time"

	"github.com/notnil/chess"
	"github.com/sethvargo/go-retry"
	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func DoRetry(description string, fn func() (interface{}, error)) (interface{}, error) {
	var err error
	b, err := retry.NewFibonacci(1 * time.Second)
	if err != nil {
		panic(err)
	}
	b = retry.WithMaxRetries(5, b)
	for {
		log.Info("trying: ", description)
		var val interface{}
		val, err = fn()
		if err == nil {
			log.Info(description, " succeeded")
			return val, nil
		}
		nextDuration, stop := b.Next()
		log.Debugf("  %s failed. Retrying in %f seconds...", description, nextDuration.Seconds())
		if stop {
			log.Debugf("  %s failed. Retry limit reached. Will not retry.", description)
			err = errors.New("%s failed. Retry limit reached. Will not retry.")
			break
		}
		time.Sleep(nextDuration)
	}
	return nil, err
}

func encryptMessageEventContent(roomID id.RoomID, eventContent *event.MessageEventContent) (event.Type, interface{}, error) {
	if !App.stateStore.IsEncrypted(roomID) {
		return event.EventMessage, eventContent, nil
	}

	log.Debugf("Encrypting event for %s", roomID)
	encrypted, err := App.olmMachine.EncryptMegolmEvent(roomID, event.EventMessage, eventContent)

	// These three errors mean we have to make a new Megolm session
	if err == crypto.SessionExpired || err == crypto.SessionNotShared || err == crypto.NoGroupSession {
		err = App.olmMachine.ShareGroupSession(roomID, App.stateStore.GetRoomMembers(roomID))
		if err != nil {
			log.Errorf("Failed to share group session to %s: %s", roomID, err)
			return event.EventMessage, eventContent, err
		}

		encrypted, err = App.olmMachine.EncryptMegolmEvent(roomID, event.EventMessage, eventContent)
	}

	if err != nil {
		log.Errorf("Failed to encrypt message to %s: %s", roomID, err)
		return event.EventMessage, eventContent, err
	}

	encrypted.RelatesTo = eventContent.RelatesTo // The m.relates_to field should be unencrypted, so copy it.

	return event.EventEncrypted, encrypted, nil
}

func SendMessage(roomId id.RoomID, content *event.MessageEventContent) (resp *mautrix.RespSendEvent, err error) {
	r, err := DoRetry(fmt.Sprintf("send message to %s", roomId), func() (interface{}, error) {
		eventType, encrypted, err := encryptMessageEventContent(roomId, content)
		if err != nil {
			return nil, err
		}
		return App.client.SendMessageEvent(roomId, eventType, encrypted)
	})
	if err != nil {
		// give up
		log.Errorf("Failed to send message to %s: %s", roomId, err)
		return nil, err
	}
	return r.(*mautrix.RespSendEvent), err
}

func SendBoardImage(roomID id.RoomID, board *chess.Board, replyingTo *id.EventID, squares ...chess.Square) (*mautrix.RespSendEvent, error) {
	pngBytes, err := boardToPngBytes(board, squares...)
	if err != nil {
		return nil, err
	}

	upload, err := App.client.UploadBytesWithName(pngBytes, "image/png", "chessboard.png")
	if err != nil {
		return nil, err
	}

	content := event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "chessboard.png",
		URL:     upload.ContentURI.CUString(),
	}

	if replyingTo != nil {
		content.SetRelatesTo(&event.RelatesTo{
			Type:    event.RelationType("m.thread"),
			EventID: *replyingTo,
		})
	}

	r, err := DoRetry(fmt.Sprintf("send chess board image to %s", roomID), func() (interface{}, error) {
		eventType, encrypted, err := encryptMessageEventContent(roomID, &content)
		if err != nil {
			return nil, err
		}
		return App.client.SendMessageEvent(roomID, eventType, encrypted)
	})

	if err != nil {
		// give up
		log.Errorf("Failed to send message to %s: %s", roomID, err)
		return nil, err
	}
	return r.(*mautrix.RespSendEvent), err
}

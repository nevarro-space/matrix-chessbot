package main

import (
	"errors"
	"fmt"
	"image/color"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/notnil/chess"
	cimage "github.com/notnil/chess/image"
	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	mevent "maunium.net/go/mautrix/event"
	mid "maunium.net/go/mautrix/id"
)

func sendHelp(roomId mid.RoomID) {
	// send message to channel confirming join (retry 3 times)
	noticeText := `COMMANDS:
* new -- start a new game of chess
* help -- show this help

Version %s. Source code: https://github.com/sumnerevans/matrix-chessbot`
	noticeHtml := `<b>COMMANDS:</b>
<ul>
<li><b>new</b> &mdash; start a new game of chess</li>
<li><b>help</b> &mdash; show this help</li>
</ul>

Version %s. <a href="https://github.com/sumnerevans/matrix-chessbot">Source code</a>.`

	SendMessage(roomId, &mevent.MessageEventContent{
		MsgType:       mevent.MsgNotice,
		Body:          fmt.Sprintf(noticeText, VERSION),
		Format:        mevent.FormatHTML,
		FormattedBody: fmt.Sprintf(noticeHtml, VERSION),
	})
}

func getCommandParts(body string) ([]string, error) {
	userId := mid.UserID(App.configuration.Username)
	localpart, _, _ := userId.ParseAndDecode()

	// Valid command strings include:
	// chessbot: foo
	// !chess foo
	// !chessbot foo
	// @chessbot foo
	// @chessbot: foo

	validCommandRegexes := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf("^%s:(.*)$", localpart)),
		regexp.MustCompile(fmt.Sprintf("^@%s:?(.*)$", localpart)),
		regexp.MustCompile("^!chessbot$"),
		regexp.MustCompile("^!chessbot:? (.*)$"),
		regexp.MustCompile("^!chess$"),
		regexp.MustCompile("^!chess:? (.*)$"),
	}

	body = strings.TrimSpace(body)

	isCommand := false
	commandParts := []string{}
	for _, commandRe := range validCommandRegexes {
		match := commandRe.FindStringSubmatch(body)
		if match != nil {
			isCommand = true
			if len(match) > 1 {
				commandParts = strings.Split(match[1], " ")
			} else {
				commandParts = []string{"help"}
			}
			break
		}
	}
	if !isCommand {
		return []string{}, errors.New("Not a command")
	}

	return commandParts, nil
}

func boardToPngBytes(board *chess.Board, squares ...chess.Square) ([]byte, error) {
	svgTempfile, err := os.CreateTemp(os.TempDir(), "chessboard-*.svg")
	if err != nil {
		return []byte{}, err
	}
	defer os.Remove(svgTempfile.Name())

	// write board SVG to file
	yellow := color.RGBA{255, 255, 0, 1}
	mark := cimage.MarkSquares(yellow, squares...)
	if err := cimage.SVG(svgTempfile, board, mark); err != nil {
		log.Fatal(err)
	}

	pngTempfile, err := os.CreateTemp(os.TempDir(), "chessboard-*.png")
	if err != nil {
		return []byte{}, err
	}
	defer os.Remove(pngTempfile.Name())

	cmd := exec.Command("convert", svgTempfile.Name(), pngTempfile.Name())
	err = cmd.Run()
	if err != nil {
		return []byte{}, err
	}

	pngFile, err := os.Open(pngTempfile.Name())
	defer pngFile.Close()
	if err != nil {
		return []byte{}, err
	}

	return ioutil.ReadAll(pngFile)
}

func sendBoardImage(roomID mid.RoomID, board *chess.Board, squares ...chess.Square) (*mautrix.RespSendEvent, error) {
	pngBytes, err := boardToPngBytes(board)
	if err != nil {
		return nil, err
	}

	upload, err := App.client.UploadBytesWithName(pngBytes, "image/png", "chessboard.png")
	if err != nil {
		return nil, err
	}
	return App.client.SendImage(roomID, "chessboard.png", upload.ContentURI)
}

var StateChessGame = mevent.Type{Type: "space.nevarro.chess.game", Class: mevent.StateEventType}

type StateChessGameEventContent struct {
	PGN               string
	BoardImageEventID mid.EventID
}

func saveGame(roomID mid.RoomID, game *chess.Game, boardImageEventID mid.EventID) (resp *mautrix.RespSendEvent, err error) {
	return App.client.SendStateEvent(roomID, StateChessGame, "", StateChessGameEventContent{
		PGN:               game.String(),
		BoardImageEventID: boardImageEventID,
	})
}

func getGameStateEvent(roomID mid.RoomID) (*StateChessGameEventContent, error) {
	var chessGame StateChessGameEventContent
	err := App.client.StateEvent(roomID, StateChessGame, "", &chessGame)
	if err != nil {
		return nil, err
	}
	return &chessGame, nil
}

func handleCommand(source mautrix.EventSource, event *mevent.Event, commandParts []string) {
	switch strings.ToLower(commandParts[0]) {
	case "new":
		log.Info(commandParts)
		game := chess.NewGame()
		game.AddTagPair("Event", fmt.Sprintf("%s @ %s", event.RoomID.String(), time.Now()))
		boardImageEvent, err := sendBoardImage(event.RoomID, game.Position().Board())
		if err == nil {
			saveGame(event.RoomID, game, boardImageEvent.EventID)
		}

		break
	default:
		sendHelp(event.RoomID)
		break
	}
}

func HandleMessage(source mautrix.EventSource, event *mevent.Event) {
	if event.Sender.String() == App.configuration.Username {
		log.Infof("Event %s is from us, so not going to respond.", event.ID)
		return
	}

	// Mark the message as read after we've handled it.
	defer App.client.MarkRead(event.RoomID, event.ID)

	messageEventContent := event.Content.AsMessage()
	commandParts, err := getCommandParts(messageEventContent.Body)

	if err == nil {
		handleCommand(source, event, commandParts)
	} else {
		gameStateEvent, err := getGameStateEvent(event.RoomID)
		pgn, err := chess.PGN(strings.NewReader(gameStateEvent.PGN))
		if err != nil {
			return
		}
		game := chess.NewGame(pgn)

		if err != nil {
			return
		}
		if err = game.MoveStr(messageEventContent.Body); err != nil {
			return
		}
		moves := game.Moves()
		last := moves[len(moves)-1]

		App.client.RedactEvent(event.RoomID, gameStateEvent.BoardImageEventID)
		resp, err := sendBoardImage(event.RoomID, game.Position().Board(), last.S1(), last.S2())
		if err != nil {
			return
		}
		_, err = saveGame(event.RoomID, game, resp.EventID)
		if err != nil {
			return
		}
	}
}

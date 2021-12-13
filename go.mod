module github.com/sumnerevans/matrix-chessbot

go 1.16

replace github.com/notnil/chess => github.com/sumnerevans/chess v1.6.0-sumner

require (
	github.com/mattn/go-sqlite3 v1.14.9
	github.com/notnil/chess v1.6.0
	github.com/sethvargo/go-retry v0.1.0
	github.com/sirupsen/logrus v1.8.1
	gopkg.in/yaml.v2 v2.4.0
	maunium.net/go/mautrix v0.10.6
)

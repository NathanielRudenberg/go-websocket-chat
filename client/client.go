package main

import (
	"fmt"
	"sync"
	connectionservice "websocket-chat/client/connection-service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	message          string
	app              *tview.Application = tview.NewApplication()
	chatMessageInput *tview.InputField  = tview.NewInputField()
	messageChannel                      = make(chan string)
)

func handleSendMessage(key tcell.Key) {
	fmt.Println("key:", key)
	switch key {
	case tcell.KeyEnter:
		// send message
		connectionservice.SendChat(message)
		yourMessage := fmt.Sprintf("[green]You[white]: %s", message)
		messageChannel <- yourMessage
		chatMessageInput.SetText("")
	}
}

func handleChangeInput(txt string) {
	message = txt
}

func handleChangeTextView() {
	app.Draw()
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		connectionservice.ConnectToChatServer(&messageChannel)
	}()

	chatWindow := tview.NewTextView().
		SetChangedFunc(handleChangeTextView).
		SetScrollable(false).
		SetDynamicColors(true)

	go func() {
		for {
			message := <-messageChannel
			fmt.Fprintf(chatWindow, "%s\n\n", message)
		}
	}()

	chatMessageInput.
		SetLabel("Message: ").
		SetFieldWidth(0).
		SetChangedFunc(handleChangeInput).
		SetDoneFunc(handleSendMessage)

	mainView := tview.NewGrid().
		SetRows(0, 3).
		SetBorders(false).
		AddItem(chatWindow, 0, 0, 1, 1, 0, 0, false).
		AddItem(chatMessageInput, 1, 0, 1, 1, 0, 0, true)

	chatWindow.
		SetBorder(true).
		SetTitle("Go Websocket Chat Demo")

	if err := app.SetRoot(mainView, true).EnableMouse(false).Run(); err != nil {
		panic(err)
	}

	// Wait for the goroutine to finish when the user exits the chat
	wg.Wait()
}

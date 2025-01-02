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
	chatChannel                         = make(chan string)
	closeChannel                        = make(chan struct{})
)

func handleSendMessage(key tcell.Key) {
	fmt.Println("key:", key)
	switch key {
	case tcell.KeyEnter:
		// send message
		if message != "" {
			connectionservice.SendChat(message)
			yourMessage := fmt.Sprintf("[green]You[white]: %s", message)
			chatChannel <- yourMessage
			chatMessageInput.SetText("")
		}
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
		connectionservice.ConnectToChatServer(&chatChannel, &closeChannel)
	}()

	chatWindow := tview.NewTextView().
		SetChangedFunc(handleChangeTextView).
		SetScrollable(false).
		SetDynamicColors(true)

	go func() {
		for {
			message := <-chatChannel
			fmt.Fprintf(chatWindow, "%s\n\n", message)
		}
	}()

	chatMessageInput.
		SetPlaceholder("Send a message...").
		SetPlaceholderTextColor(tcell.ColorLightGray).
		SetPlaceholderStyle(tcell.StyleDefault.Foreground(tcell.ColorLightGray)).
		SetFieldWidth(0).
		SetChangedFunc(handleChangeInput).
		SetDoneFunc(handleSendMessage).
		SetFieldBackgroundColor(tcell.ColorBlack)

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

	// Close the connection when the user exits the chat
	closeChannel <- struct{}{}

	// Wait for the goroutine to finish when the user exits the chat
	wg.Wait()
}

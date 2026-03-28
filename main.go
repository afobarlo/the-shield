package main

import (
	//"embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"path"
	"github.com/bwmarrin/discordgo"
	"log"
	"io"
	"net/http"
)

// "HERE GO SECRET"
// Variables used for command line parameters
var (
	Token string
)

func init() {

	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + "HERE GO SECRET")
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	//var output string

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
	// If the message is "ping" reply with "Pong!"
	if m.Content == "!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
		return
	}

	// If the message is "pong" reply with "Ping!"
	if m.Content == "!pong" {
		s.ChannelMessageSend(m.ChannelID, "Ping!")
		return
	}
	
	
	if strings.Contains(m.Content, "://") {
		messageUrl := strings.Replace(m.Content, "/x.com", "/cunnyx.com", 1)
		messageUrl = strings.Replace(messageUrl, "pixiv.net", "phixiv.net", 1)
		if strings.Contains(m.Content, "tenor.com") {
			return
		}
		s.ChannelMessageSend(m.ChannelID, messageUrl)
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			log.Println("Cannot delete message: %v", err)
		}
		return
	}
	
	if m.Attachments != nil {
		if len(m.Attachments) == 0 {
			return
		}
		var fileList []string
		var fileName string
		var filePath string
		var discordFiles []*discordgo.File
		for i := range(len(m.Attachments)) {
			fileName = strings.Split(path.Base(m.Attachments[i].URL), "?")[0]
			fileList = append(fileList, fileName)
			filePath = "./" + fileName
			resp, err := http.Get(m.Attachments[i].URL)
			if err != nil {
				log.Println(err)
				continue
			}
			defer resp.Body.Close()
			
			imgData, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
				continue
			}
			
			output, err := os.Create(filePath)
			if err != nil {
				log.Println(err)
				continue
			}
			defer output.Close()
	
			if _, err := output.Write(imgData); err != nil {
				log.Println(err)
				continue
			}
			
			file, err := os.Open(filePath)
			if err != nil {
				log.Println(err)
				continue
			}
			
			discordFiles = append(discordFiles, &discordgo.File{Name: fileName, Reader: file})
			
		}
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: m.Content, Files: discordFiles})
		
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			log.Println("Cannot delete message: %v", err)
		}
		
		for _, fileName := range fileList {
			err := os.Remove(fileName)
			if err != nil {
				log.Println(err)
			}
		}
		
		
	}
	
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	//cache "github.com/cothromachd/in-memory-cache"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	tele "gopkg.in/telebot.v3"
)

func main() {
	//c := cache.New()

	//c.Load(12 * time.Hour) // restoring clients messages from database file if host crash case happened

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	/*go func() { // save to file the state of the client message store every hour
		for range ticker.C {
			c.Store()
		}
	}()*/

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")

	}

	token, ok := os.LookupEnv("TOKEN")
	if !ok {
		log.Fatal("No TOKEN variable in .env")
	}

	schatID, ok := os.LookupEnv("CHAT_ID")
	if !ok {
		log.Fatal("No CHAT_ID variable in .env")
	}

	chatID, err := strconv.Atoi(schatID)
	if err != nil {
		log.Fatal("Convertation CHAT_ID from string to int failed")
	}

	adminChatID := int64(chatID)

	logsFile, err := os.OpenFile("logfile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer logsFile.Close()

	logger := log.New(logsFile, "", log.Ldate|log.Ltime)

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 60 * time.Second},
		OnError: func(err error, ctx tele.Context) {
			logger.Printf("%v\n", err)

			log.Println(err)
		},
	}

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	b, err := tele.NewBot(pref)
	if err != nil {
		return
	}

	b.Handle("/start", func(ctx tele.Context) error {
		return ctx.Reply(`السلام عليكم ورحمة الله وبركاته
Я бот обратной связи. Отправьте мне свой вопрос или сообщение, и я передам его администратору.`)
	})

	b.Handle(tele.OnText, func(ctx tele.Context) error {
		if ctx.Chat().ID == adminChatID && ctx.Message().ReplyTo != nil { // in case if author text
			replyMsg := ctx.Message().ReplyTo

			var ToChatID string
			var userChatID int64

			if replyMsg != nil {
				sender := replyMsg.OriginalSender
				if sender != nil {
					userChatID = sender.ID
					log.Printf("Author to %s %d: %s", sender.Username, replyMsg.OriginalUnixtime, ctx.Message().Text)

				} else {
					//userChatIDI, err = c.Get(fmt.Sprintf("%s %s %s %d", replyMsg.OriginalSenderName, replyMsg.Text, replyMsg.Caption, replyMsg.OriginalUnixtime))
					ToChatID, err = client.Get(context.Background(), fmt.Sprintf("%s %s %s %d", replyMsg.OriginalSenderName, replyMsg.Text, replyMsg.Caption, replyMsg.OriginalUnixtime)).Result()
					if err != nil {
						return err
					}

					log.Printf("Author to %s %d: %s\n", replyMsg.OriginalSenderName, replyMsg.OriginalUnixtime, ctx.Message().Text)

					/*
						switch v := userChatIDI.(type) {
						case int64:
							chatIDToSend = v
						case float64:
							chatIDToSend = int64(v)
						default:
							return fmt.Errorf("get chatID from cache failed: can't cast interface value")
						}*/

					userChatID, err = strconv.ParseInt(ToChatID, 10, 64)
					if err != nil {
						return err
					}
				}
			}

			_, err = b.Copy(tele.ChatID(userChatID), ctx.Message())
			if err != nil {
				return err
			}

			return err
		}

		// in case if user text
		chat, err := b.ChatByID(ctx.Chat().ID)
		if err != nil {
			return err
		}

		isForbidden := chat.Private
		if isForbidden { // checking if user disallow adding a link to his account in forwarded messages
			// if so, I will save his chat_id by nickname and his text of msg to get it when admin will answer
			//c.Set(fmt.Sprintf("%s %s %s %s %d", ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Message().Text, ctx.Message().Caption, ctx.Message().Unixtime), ctx.Message().Chat.ID, 24*time.Hour)
			//c.Store()

			if ctx.Sender().LastName != "" {
				err := client.Set(context.Background(), fmt.Sprintf("%s %s %s %s %d", ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Message().Text, ctx.Message().Caption, ctx.Message().Unixtime), ctx.Message().Chat.ID, 24*time.Hour).Err()
				if err != nil {
					return err
				}
			} else if ctx.Sender().LastName == "" {
				err := client.Set(context.Background(), fmt.Sprintf("%s %s %s %d", ctx.Sender().FirstName, ctx.Message().Text, ctx.Message().Caption, ctx.Message().Unixtime), ctx.Message().Chat.ID, 24*time.Hour).Err()
				if err != nil {
					return err
				}
			}

			log.Printf("%s %s %d: %s\n", ctx.Message().Sender.FirstName, ctx.Message().Sender.LastName, ctx.Message().Unixtime, ctx.Message().Text)
		} else {
			log.Printf("%s (username) %d: %s\n", ctx.Message().Sender.Username, ctx.Message().Unixtime, ctx.Message().Text)
		}

		_, err = b.Send(tele.ChatID(adminChatID), fmt.Sprintf("Информация о пользователе:\nИмя: %s\nФамилия: %s\nUsername: @%s\nID: %d\nСообщение от пользователя:\n", ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Sender().Username, ctx.Sender().ID))
		if err != nil {
			return err
		}

		_, err = b.Forward(tele.ChatID(adminChatID), ctx.Message())
		if err != nil {
			b.Send(tele.ChatID(adminChatID), "Ошибка у пользователя: "+err.Error())
			return err
		}

		err = ctx.Reply("جزاك اللهُ خيرًا\nВаше сообщение успешно отправлено администратору.")
		if err != nil {
			log.Println(err)
		}

		return err
	})

	b.Handle(tele.OnMedia, func(ctx tele.Context) error {
		if ctx.Chat().ID == adminChatID && ctx.Message().ReplyTo != nil { // in case if admin text
			var ToChatID string
			var userChatID int64

			replyMsg := ctx.Message().ReplyTo

			if replyMsg != nil {
				sender := replyMsg.OriginalSender
				if sender != nil {
					userChatID = sender.ID
					log.Printf("Author to %s %d: %s\n", sender.Username, replyMsg.OriginalUnixtime, ctx.Message().Caption)
				} else {
					log.Printf("Author to %s %d: %s\n", replyMsg.OriginalSenderName, replyMsg.OriginalUnixtime, ctx.Message().Caption)

					//ToChatID, err = c.Get(fmt.Sprintf("%s %s %s %d", replyMsg.OriginalSenderName, replyMsg.Text, replyMsg.Caption, replyMsg.OriginalUnixtime))
					ToChatID, err = client.Get(context.Background(), fmt.Sprintf("%s %s %s %d", replyMsg.OriginalSenderName, replyMsg.Text, replyMsg.Caption, replyMsg.OriginalUnixtime)).Result()
					if err != nil {
						return err
					}

					/*switch v := ToChatID.(type) {
					case int64:
						userChatID = v
					case float64:
						userChatID = int64(v)
					default:
						return fmt.Errorf("get chatID from cache failed: can't cast interface value")
					}*/

					userChatID, err = strconv.ParseInt(ToChatID, 10, 64)
					if err != nil {
						return err
					}
				}
			}

			_, err = b.Copy(tele.ChatID(userChatID), ctx.Message())
			if err != nil {
				return err
			}

			return err
		}

		// in case if user text
		chat, err := b.ChatByID(ctx.Chat().ID)
		if err != nil {
			return err
		}

		isForbidden := chat.Private
		if isForbidden { // checking if user disallow adding a link to his account in forwarded messages
			// if so, I will save his chat_id by nickname and his text of msg to get it when admin will answer
			//c.Set(fmt.Sprintf("%s %s %s %s %d", ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Message().Text, ctx.Message().Caption, ctx.Message().Unixtime), ctx.Message().Chat.ID, 24*time.Hour)
			//c.Store()

			if ctx.Sender().LastName != "" {
				err := client.Set(context.Background(), fmt.Sprintf("%s %s %s %s %d", ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Message().Text, ctx.Message().Caption, ctx.Message().Unixtime), ctx.Message().Chat.ID, 24*time.Hour).Err()
				if err != nil {
					return err
				}
			} else if ctx.Sender().LastName == "" {
				err := client.Set(context.Background(), fmt.Sprintf("%s %s %s %d", ctx.Sender().FirstName, ctx.Message().Text, ctx.Message().Caption, ctx.Message().Unixtime), ctx.Message().Chat.ID, 24*time.Hour).Err()
				if err != nil {
					return err
				}
			}

			log.Printf("%s %s %d: %s\n", ctx.Message().Sender.FirstName, ctx.Message().Sender.LastName, ctx.Message().Unixtime, ctx.Message().Caption)
		} else {
			log.Printf("%s (username) %d: %s\n", ctx.Message().Sender.Username, ctx.Message().Unixtime, ctx.Message().Caption)
		}

		b.Send(tele.ChatID(adminChatID), fmt.Sprintf("Информация о пользователе:\nИмя: %s\nФамилия: %s\nUsername: @%s\nID: %d\nСообщение от пользователя:\n", ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Sender().Username, ctx.Sender().ID))
		err = ctx.Reply("جزاك اللهُ خيرًا\nВаше сообщение успешно отправлено администратору.")
		if err != nil {
			return err
		}

		return ctx.ForwardTo(tele.ChatID(adminChatID))
	})

	log.Printf("Authorized on account %s", b.Me.Username)
	b.Start()
}

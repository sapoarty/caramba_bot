package main

import (
    "crypto/tls"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"
    "io/ioutil"
    "encoding/json"
    "os"
    "errors"
    "strings"
    "github.com/joho/godotenv"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
    mu     sync.RWMutex
    chatIDs []int64
    tokenBot string
    webSite string
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }
    tokenBot = os.Getenv("carambaBotToken")
    webSite = os.Getenv("carambaBotWebSite")
    chatIDs = loadChatIDs()
    println("Бот готов к работе")
    go getUpdates() // Горутина для получения обновлений

    ticker := time.NewTicker(10 * time.Second)
    for {
        select {
        case <-ticker.C:
            statusCode, err := checkWebsite(webSite)
            if (err != nil) {
                sendTelegram(fmt.Sprintf("Аарр, капитан! Ошибка отбросила %s обратно на Берег: %v", webSite, err))            
            }
            if (err == nil) {
                if (err != nil) {
                    sendTelegram(fmt.Sprintf("Ахой, каптан! Ошибка выбросила %s обратно на материк: %v", webSite, err))
                    continue
                }
                if (statusCode != http.StatusOK) {
                    sendTelegram(fmt.Sprintf("Эй, пираты! %s заблудился в бурях, встретил Кракена! Статус: %v", webSite, statusCode))
                }
            }
        }
    }
}

func checkWebsite(webSite string) (int, error) {
    tr := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }

    client := &http.Client{Transport: tr}
    resp, err := client.Get(webSite)
    if err != nil {
        return 0, err
    }
    return resp.StatusCode, nil
}

func sendTelegram(message string) {
    println(message)
    bot, err := tgbotapi.NewBotAPI(tokenBot)
    if err != nil {
        log.Panic(err)
    }

    mu.Lock() // Lock for write
    for i := 0; i < len(chatIDs); {
        chatID := chatIDs[i]
        msg := tgbotapi.NewMessage(chatID, message)
        _, err = bot.Send(msg)

        var apiErr tgbotapi.Error
        if errors.As(err, &apiErr) {
            if strings.Contains(apiErr.Message, "Forbidden: bot was kicked from the group chat") {
                deleteChatID(chatID) // Delete chat ID from json file

                // Delete chat ID from current slice
                chatIDs = append(chatIDs[:i], chatIDs[i+1:]...)
                continue
            }
        } else if err != nil {
            log.Panic(err)  // log.Panic will call os.Exit after logging the error message
        }
        i++  // increment index only if chat id is not deleted
    }
    mu.Unlock() // Unlock
}


func getUpdates() {
    var msgText string
    bot, err := tgbotapi.NewBotAPI(tokenBot)
    if err != nil {
        log.Panic(err)
    }

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates, err := bot.GetUpdatesChan(u)
    for update := range updates {
        if update.Message == nil {
            continue
        }

        mu.Lock() // Lock for write
        chatID := update.Message.Chat.ID
        if !contains(chatIDs, chatID) {
            chatIDs = append(chatIDs, chatID)
            saveChatIDs(chatIDs) // Save ChatID list into file
        }

        if update.Message.IsCommand() {
            switch update.Message.Command() {
            case "status":
                statusCode, err := checkWebsite(webSite)
                if (err != nil) {
                    msgText = fmt.Sprintf("Аарр, капитан! Ошибка отбросила %s обратно на Берег: %v", webSite, err);
                }
                if (err == nil) {
                    if (statusCode == http.StatusOK) {
                        msgText = fmt.Sprintf("%s стоит крепко на волнах, курс на семь ветров. Статус: %v", webSite, statusCode);
                    } else {
                        msgText = fmt.Sprintf("Карамба! %s заблудился в бурях, встретил Кракена! Статус: %v", webSite, statusCode);
                    }
                }
                msg := tgbotapi.NewMessage(chatID, msgText)
                bot.Send(msg)
                break

            case "start":
            case "help":
                msgText := fmt.Sprintf(
                    "Аааррр! Я корсарский бот-сторож, моряк!\n\n" +
                    "/status - Получить текущее состояние нашего корабля.\n" +
                    "/help - Если тебе нужна помощь в море.\n\n" +
                    "С интервалом в 10 секунд я проверяю, не взял ли наш сайт на абордаж какой-нибудь грязный кибер-пират или Кракен.\n" +
                    "Берегись, и держи руку на шпаге!",
                )
                msg := tgbotapi.NewMessage(chatID, msgText)
                bot.Send(msg)
            }
        }

        mu.Unlock() // Unlock
    }
}

func contains(s []int64, e int64) bool {
    for _, a := range s {
        if a == e {
            return true
        }
    }
    return false
}

func saveChatIDs(chatIDs []int64) {
    file, _ := json.MarshalIndent(chatIDs, "", " ")
    _ = ioutil.WriteFile("chatIDs.json", file, 0644)
}


func deleteChatID(chatIDToDelete int64) {
    fmt.Println("Deleting chat id: ", chatIDToDelete)
    file, err := os.Open("chatIDs.json")
    if err != nil {
        log.Fatal("Error opening file:", err)
    }
    defer file.Close()

    bytes, _ := ioutil.ReadAll(file)
    var chatIDs []int64
    json.Unmarshal(bytes, &chatIDs)

    for i, chatID := range chatIDs {
        if chatID == chatIDToDelete {
            chatIDs = append(chatIDs[:i], chatIDs[i+1:]...)
            break
        }
    }

    data, _ := json.MarshalIndent(chatIDs, "", " ")
    _ = ioutil.WriteFile("chatIDs.json", data, 0644)
}

func loadChatIDs() []int64 {
    filename := "chatIDs.json"
    _, err := os.Stat(filename) // Проверка существования файла
    if os.IsNotExist(err) { // Если файла нет, создаем его
        saveChatIDs(chatIDs) // Создаст пустой файл chatIDs.json
    }

    file, err := os.Open("chatIDs.json") // Открываем файл
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    // Считываем содержимое файла
    byteValue, err := ioutil.ReadAll(file)
    if err != nil {
        log.Fatal(err)
    }

    err = json.Unmarshal(byteValue, &chatIDs) // Декодируем JSON в срез
    if err != nil {
        log.Fatal(err)
    }

    return chatIDs
}

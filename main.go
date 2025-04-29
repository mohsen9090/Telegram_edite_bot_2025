
package main

import (
"fmt"
"image"
"image/color"
"io"
"log"
"net/http"
"os"
"path/filepath"
"time"

"github.com/fogleman/gg"
"github.com/disintegration/imaging"
tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const BotToken = "8022569127:AAHeGgV3SuaiM87_ooBzR7AVJoTjqVf039w" // Replace with your actual bot token

type UserSession struct {
State         string
ImagePath     string
OriginalPath  string
Settings      ImageSettings
LastMenuMsgID int
}

type ImageSettings struct {
TextSize  float64
FontColor color.Color
Quality   int
}

var (
bot           *tgbotapi.BotAPI
userSessions  = make(map[int64]*UserSession)
)

func init() {
var err error
bot, err = tgbotapi.NewBotAPI(BotToken)
if err != nil {
log.Fatal(err)
}

bot.Debug = false
log.Printf("Bot started: @%s", bot.Self.UserName)

// Run file cleanup every hour
go func() {
ticker := time.NewTicker(1 * time.Hour)
for range ticker.C {
cleanOldFiles()
}
}()
}

func getSession(chatID int64) *UserSession {
session, exists := userSessions[chatID]
if !exists {
session = &UserSession{
State: "normal",
Settings: ImageSettings{
TextSize:  48,
FontColor: color.White,
Quality:   90,
},
}
userSessions[chatID] = session
}
return session
}

func createDirs() {
dirs := []string{"downloads", "processed"}
for _, dir := range dirs {
if err := os.MkdirAll(dir, 0755); err != nil {
log.Printf("Error creating directory %s: %v", dir, err)
}
}
}

func handleUpdate(update tgbotapi.Update) {
var chatID int64
if update.Message != nil {
chatID = update.Message.Chat.ID
} else if update.CallbackQuery != nil {
chatID = update.CallbackQuery.Message.Chat.ID
} else {
return
}

session := getSession(chatID)

if update.Message != nil {
handleMessage(update.Message, session)
} else if update.CallbackQuery != nil {
handleCallback(update.CallbackQuery, session)
}
}

func handleMessage(message *tgbotapi.Message, session *UserSession) {
if message.Photo != nil {
handlePhoto(message, session)
return
}

if message.Text == "/start" {
sendWelcomeMessage(message.Chat.ID)
sendMainMenu(message.Chat.ID, session)
return
}

if session.State == "waiting_text" && message.Text != "" {
handleText(message, session)
return
}

switch message.Text {
case "/mft":
showProEffectMenu(message.Chat.ID, session)
case "/text":
session.State = "waiting_text"
msg := tgbotapi.NewMessage(message.Chat.ID, "✏️ Enter your text:")
bot.Send(msg)
case "/resize":
showResizeMenu(message.Chat.ID, session)
case "/effect":
showEffectMenu(message.Chat.ID, session)
case "/next":
showNextMenu(message.Chat.ID, session)
case "/gallery_effects":
showEffectsGallery(message.Chat.ID, session)
case "/gallery_size":
showSizeGallery(message.Chat.ID, session)
case "/gallery_frames":
showFramesGallery(message.Chat.ID, session)
case "/gallery_tools":
showToolsGallery(message.Chat.ID, session)
case "/reset":
if session.OriginalPath != "" {
session.ImagePath = session.OriginalPath
msg := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FilePath(session.OriginalPath))
msg.Caption = "🔄 Image reset to original"
bot.Send(msg)
sendMainMenu(message.Chat.ID, session)
}
default:
sendError(message.Chat.ID, "Unknown command")
}
}

func handleCallback(callback *tgbotapi.CallbackQuery, session *UserSession) {
chatID := callback.Message.Chat.ID
data := callback.Data

if session.LastMenuMsgID != 0 {
deleteMsg := tgbotapi.NewDeleteMessage(chatID, session.LastMenuMsgID)
bot.Send(deleteMsg)
}

switch data {
case "show_pro":
showProEffectMenu(chatID, session)
case "show_text":
session.State = "waiting_text"
msg := tgbotapi.NewMessage(chatID, "✏️ Enter your text:")
bot.Send(msg)
case "show_resize":
showResizeMenu(chatID, session)
case "show_effect":
showEffectMenu(chatID, session)
case "show_next":
showNextMenu(chatID, session)
case "gallery_effects":
showEffectsGallery(chatID, session)
case "gallery_size":
showSizeGallery(chatID, session)
case "gallery_frames":
showFramesGallery(chatID, session)
case "gallery_tools":
showToolsGallery(chatID, session)
case "back_main":
sendMainMenu(chatID, session)
case "back_next":
showNextMenu(chatID, session)
case "effect_bw", "effect_bright", "effect_dark":
applyEffect(chatID, session, data)
sendMainMenu(chatID, session)
case "effect_hdr", "effect_shine", "effect_portrait", "effect_sunset", "effect_paint", "effect_photo":
applyProEffect(chatID, session, data)
sendMainMenu(chatID, session)
case "frame_purple", "frame_green", "frame_blue", "frame_red", "frame_gold", "frame_white":
applyFrame(chatID, session, data)
sendMainMenu(chatID, session)
case "size_small", "size_medium", "size_large":
resizeImage(chatID, session, data)
sendMainMenu(chatID, session)
case "size_insta_post", "size_insta_story", "size_twitter", "size_facebook", "size_telegram", "size_profile":
applySocialResize(chatID, session, data)
sendMainMenu(chatID, session)
case "reset":
if session.OriginalPath != "" {
session.ImagePath = session.OriginalPath
msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(session.OriginalPath))
msg.Caption = "🔄 Image reset to original"
bot.Send(msg)
sendMainMenu(chatID, session)
}
}

bot.Send(tgbotapi.NewCallback(callback.ID, ""))
}

func handlePhoto(message *tgbotapi.Message, session *UserSession) {
photo := message.Photo[len(message.Photo)-1]
fileID := photo.FileID
file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
if err != nil {
sendError(message.Chat.ID, "Error receiving file")
return
}

fileName := fmt.Sprintf("downloads/%d%d.jpg", message.Chat.ID, time.Now().Unix())
err = downloadFile(file.Link(bot.Token), fileName)
if err != nil {
sendError(message.Chat.ID, "Error saving file")
return
}

session.ImagePath = fileName
session.OriginalPath = fileName
session.State = "normal"

msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Image uploaded successfully")
bot.Send(msg)
sendMainMenu(message.Chat.ID, session)
}

func sendMainMenu(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🎨 Pro Effect", "show_pro"),
tgbotapi.NewInlineKeyboardButtonData("✏️ Add Text", "show_text"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("📏 Resize", "show_resize"),
tgbotapi.NewInlineKeyboardButtonData("🎭 Effect", "show_effect"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("➡️ Next", "show_next"),
),
)

msg := tgbotapi.NewMessage(chatID, "🎯 Select Operation:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showProEffectMenu(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🌈 HDR", "effect_hdr"),
tgbotapi.NewInlineKeyboardButtonData("✨ Shine", "effect_shine"),
tgbotapi.NewInlineKeyboardButtonData("🖼 Portrait", "effect_portrait"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🌅 Sunset", "effect_sunset"),
tgbotapi.NewInlineKeyboardButtonData("🎨 Paint", "effect_paint"),
tgbotapi.NewInlineKeyboardButtonData("📷 Photo", "effect_photo"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_main"),
),
)

msg := tgbotapi.NewMessage(chatID, "🎨 Select Pro Effect:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showNextMenu(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🎨 Effects Gallery", "gallery_effects"),
tgbotapi.NewInlineKeyboardButtonData("📏 Size Gallery", "gallery_size"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🖼 Frames Gallery", "gallery_frames"),
tgbotapi.NewInlineKeyboardButtonData("✨ Tools Gallery", "gallery_tools"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_main"),
),
)

msg := tgbotapi.NewMessage(chatID, "🎯 Select Gallery:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showEffectMenu(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("⚫️ Black & White", "effect_bw"),
tgbotapi.NewInlineKeyboardButtonData("☀️ Bright", "effect_bright"),
tgbotapi.NewInlineKeyboardButtonData("🌙 Dark", "effect_dark"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_main"),
),
)

msg := tgbotapi.NewMessage(chatID, "🎨 Select basic effect:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showResizeMenu(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("Small (800px)", "size_small"),
tgbotapi.NewInlineKeyboardButtonData("Medium (1200px)", "size_medium"),
tgbotapi.NewInlineKeyboardButtonData("Large (1600px)", "size_large"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_main"),
),
)

msg := tgbotapi.NewMessage(chatID, "📏 Select size:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showEffectsGallery(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🌈 HDR", "effect_hdr"),
tgbotapi.NewInlineKeyboardButtonData("✨ Shine", "effect_shine"),
tgbotapi.NewInlineKeyboardButtonData("🖼 Portrait", "effect_portrait"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🌅 Sunset", "effect_sunset"),
tgbotapi.NewInlineKeyboardButtonData("🎨 Paint", "effect_paint"),
tgbotapi.NewInlineKeyboardButtonData("📷 Photo", "effect_photo"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("⚫️ B&W", "effect_bw"),
tgbotapi.NewInlineKeyboardButtonData("☀️ Bright", "effect_bright"),
tgbotapi.NewInlineKeyboardButtonData("🌙 Dark", "effect_dark"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_next"),
),
)

msg := tgbotapi.NewMessage(chatID, "🎨 Effects Gallery - Select an effect:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showSizeGallery(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("📸 Instagram Post", "size_insta_post"),
tgbotapi.NewInlineKeyboardButtonData("📱 Story", "size_insta_story"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🐦 Twitter", "size_twitter"),
tgbotapi.NewInlineKeyboardButtonData("👥 Facebook", "size_facebook"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("💬 Telegram", "size_telegram"),
tgbotapi.NewInlineKeyboardButtonData("👤 Profile", "size_profile"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_next"),
),
)

msg := tgbotapi.NewMessage(chatID, "📏 Size Gallery - Select a size:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showFramesGallery(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🟣 Purple", "frame_purple"),
tgbotapi.NewInlineKeyboardButtonData("💚 Green", "frame_green"),
tgbotapi.NewInlineKeyboardButtonData("💙 Blue", "frame_blue"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("❤️ Red", "frame_red"),
tgbotapi.NewInlineKeyboardButtonData("💛 Gold", "frame_gold"),
tgbotapi.NewInlineKeyboardButtonData("⚪️ White", "frame_white"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_next"),
),
)

msg := tgbotapi.NewMessage(chatID, "🖼 Frames Gallery - Select frame color:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func showToolsGallery(chatID int64, session *UserSession) {
keyboard := tgbotapi.NewInlineKeyboardMarkup(
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("✏️ Add Text", "show_text"),
tgbotapi.NewInlineKeyboardButtonData("📏 Resize", "show_resize"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔄 Reset", "reset"),
tgbotapi.NewInlineKeyboardButtonData("🎨 Effects", "show_effect"),
),
tgbotapi.NewInlineKeyboardRow(
tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_next"),
),
)

msg := tgbotapi.NewMessage(chatID, "🛠 Tools Gallery - Select tool:")
msg.ReplyMarkup = keyboard
sent, _ := bot.Send(msg)
session.LastMenuMsgID = sent.MessageID
}

func applyEffect(chatID int64, session *UserSession, effectType string) {
if session.ImagePath == "" {
sendError(chatID, "Please send an image first")
return
}

src, err := imaging.Open(session.ImagePath)
if err != nil {
sendError(chatID, "Error processing image")
return
}

var img *image.NRGBA
switch effectType {
case "effect_bw":
img = imaging.Grayscale(src)
case "effect_bright":
img = imaging.AdjustBrightness(src, 30)
img = imaging.AdjustContrast(img, 10)
case "effect_dark":
img = imaging.AdjustBrightness(src, -30)
img = imaging.AdjustContrast(img, 10)
}

newPath := fmt.Sprintf("processed/%d_%s.png", time.Now().UnixNano(), effectType)
err = imaging.Save(img, newPath)
if err != nil {
sendError(chatID, "Error saving image")
return
}

session.ImagePath = newPath
msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(newPath))
msg.Caption = "✅ Effect applied successfully"
bot.Send(msg)
}

func applyProEffect(chatID int64, session *UserSession, effectType string) {
if session.ImagePath == "" {
sendError(chatID, "Please send an image first")
return
}

src, err := imaging.Open(session.ImagePath)
if err != nil {
sendError(chatID, "Error processing image")
return
}

var img *image.NRGBA
switch effectType {
case "effect_hdr":
img = imaging.AdjustContrast(src, 25)
img = imaging.AdjustBrightness(img, 5)
img = imaging.AdjustSaturation(img, 35)
img = imaging.Sharpen(img, 2)
case "effect_shine":
img = imaging.AdjustBrightness(src, 15)
img = imaging.AdjustContrast(img, 15)
img = imaging.AdjustGamma(img, 0.9)
case "effect_portrait":
img = imaging.Blur(src, 0.5)
img = imaging.AdjustContrast(img, 15)
img = imaging.AdjustSaturation(img, 10)
case "effect_sunset":
img = imaging.AdjustGamma(src, 0.85)
img = imaging.AdjustSaturation(img, 25)
img = imaging.AdjustBrightness(img, -5)
case "effect_paint":
img = imaging.Blur(src, 1)
img = imaging.Sharpen(img, 3)
img = imaging.AdjustSaturation(img, 30)
case "effect_photo":
img = imaging.Sharpen(src, 1)
img = imaging.AdjustContrast(img, 20)
img = imaging.AdjustBrightness(img, 5)
}

newPath := fmt.Sprintf("processed/%d_%s.png", time.Now().UnixNano(), effectType)
err = imaging.Save(img, newPath)
if err != nil {
sendError(chatID, "Error saving image")
return
}

session.ImagePath = newPath
msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(newPath))
msg.Caption = "✅ Pro effect applied successfully"
bot.Send(msg)
}

func applyFrame(chatID int64, session *UserSession, frameType string) {
if session.ImagePath == "" {
sendError(chatID, "Please send an image first")
return
}

src, err := imaging.Open(session.ImagePath)
if err != nil {
sendError(chatID, "Error processing image")
return
}

var frameColor color.Color
frameSize := 20

switch frameType {
case "frame_purple":
frameColor = color.RGBA{128, 0, 128, 255}
case "frame_green":
frameColor = color.RGBA{0, 128, 0, 255}
case "frame_blue":
frameColor = color.RGBA{0, 0, 255, 255}
case "frame_red":
frameColor = color.RGBA{255, 0, 0, 255}
case "frame_gold":
frameColor = color.RGBA{255, 215, 0, 255}
case "frame_white":
frameColor = color.RGBA{255, 255, 255, 255}
}

bounds := src.Bounds()
dst := imaging.New(bounds.Dx()+frameSize*2, bounds.Dy()+frameSize*2, frameColor)
dst = imaging.Paste(dst, src, image.Point{frameSize, frameSize})

newPath := fmt.Sprintf("processed/%d_%s.png", time.Now().UnixNano(), frameType)
err = imaging.Save(dst, newPath)
if err != nil {
sendError(chatID, "Error saving image")
return
}

session.ImagePath = newPath
msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(newPath))
msg.Caption = "✅ Frame added successfully"
bot.Send(msg)
}

func handleText(message *tgbotapi.Message, session *UserSession) {
if session.ImagePath == "" {
sendError(message.Chat.ID, "Please send an image first")
return
}

img, err := gg.LoadImage(session.ImagePath)
if err != nil {
sendError(message.Chat.ID, "Error processing image")
return
}

dc := gg.NewContextForImage(img)

if err := dc.LoadFontFace("font.ttf", 48); err != nil {
sendError(message.Chat.ID, "Error loading font")
return
}

textWidth, textHeight := dc.MeasureString(message.Text)
x := float64(dc.Width())/2 - textWidth/2
y := float64(dc.Height())/2 + textHeight/2

// Text shadow
dc.SetRGB(0, 0, 0)
dc.DrawString(message.Text, x+2, y+2)

// Main text
dc.SetRGB(1, 1, 1)
dc.DrawString(message.Text, x, y)

newPath := fmt.Sprintf("processed/%d_text.png", time.Now().UnixNano())
if err := dc.SavePNG(newPath); err != nil {
sendError(message.Chat.ID, "Error saving image")
return
}

session.ImagePath = newPath
session.State = "normal"

msg := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FilePath(newPath))
msg.Caption = "✅ Text added successfully"
bot.Send(msg)

sendMainMenu(message.Chat.ID, session)
}

func applySocialResize(chatID int64, session *UserSession, sizeType string) {
if session.ImagePath == "" {
sendError(chatID, "Please send an image first")
return
}

src, err := imaging.Open(session.ImagePath)
if err != nil {
sendError(chatID, "Error processing image")
return
}

var width, height int
switch sizeType {
case "size_insta_post":
width, height = 1080, 1080
case "size_insta_story":
width, height = 1080, 1920
case "size_twitter":
width, height = 1200, 675
case "size_facebook":
width, height = 1200, 630
case "size_telegram":
width, height = 1280, 720
case "size_profile":
width, height = 500, 500
}

img := imaging.Resize(src, width, height, imaging.Lanczos)
newPath := fmt.Sprintf("processed/%d_%s.png", time.Now().UnixNano(), sizeType)
err = imaging.Save(img, newPath)
if err != nil {
sendError(chatID, "Error saving image")
return
}

session.ImagePath = newPath
msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(newPath))
msg.Caption = fmt.Sprintf("✅ Image resized to %dx%d", width, height)
bot.Send(msg)
}

func resizeImage(chatID int64, session *UserSession, sizeType string) {
if session.ImagePath == "" {
sendError(chatID, "Please send an image first")
return
}

src, err := imaging.Open(session.ImagePath)
if err != nil {
sendError(chatID, "Error processing image")
return
}

var width int
switch sizeType {
case "size_small":
width = 800
case "size_medium":
width = 1200
case "size_large":
width = 1600
}

img := imaging.Resize(src, width, 0, imaging.Lanczos)
newPath := fmt.Sprintf("processed/%d_%s.png", time.Now().UnixNano(), sizeType)
err = imaging.Save(img, newPath)
if err != nil {
sendError(chatID, "Error saving image")
return
}

session.ImagePath = newPath
msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(newPath))
msg.Caption = fmt.Sprintf("✅ Image resized to %dpx width", width)
bot.Send(msg)
}

func sendWelcomeMessage(chatID int64) {
text := "🌟 Welcome to VIP Image Editor Bot!  Features: ✅ Add text to images ✅ Apply professional effects ✅ Add colored frames ✅ Resize images ✅ Optimize for social media ✅ Multiple galleries with various tools  🔸 To start, please send an image."

msg := tgbotapi.NewMessage(chatID, text)
bot.Send(msg)
}

func sendError(chatID int64, text string) {
msg := tgbotapi.NewMessage(chatID, "❌ "+text)
bot.Send(msg)
}

func downloadFile(url, filepath string) error {
resp, err := http.Get(url)
if err != nil {
return err
}
defer resp.Body.Close()

out, err := os.Create(filepath)
if err != nil {
return err
}
defer out.Close()

_, err = io.Copy(out, resp.Body)
return err
}

func cleanOldFiles() {
threshold := time.Now().Add(-24 * time.Hour)

dirs := []string{"downloads", "processed"}
for _, dir := range dirs {
files, err := os.ReadDir(dir)
if err != nil {
continue
}

for _, file := range files {
info, err := file.Info()
if err != nil {
continue
}

if info.ModTime().Before(threshold) {
os.Remove(filepath.Join(dir, file.Name()))
}
}
}
}

func main() {
createDirs()

u := tgbotapi.NewUpdate(0)
u.Timeout = 60

updates := bot.GetUpdatesChan(u) // ← فقط یک متغیر

for update := range updates {
handleUpdate(update)
}
}


package main

import (
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed index.html
var embeddedFiles embed.FS

var db *sql.DB
var appPort = os.Getenv("APP_PORT")

// Device はデータベースのdevicesテーブルを表す構造体
type Device struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Mac  string `json:"mac"`
}

func main() {
	if appPort == "" {
		appPort = "8090"
	}

	// データベースの初期化
	var err error
	// 実行時のカレントディレクトリ(Dockerfileで/app/dataに設定)にDBファイルが作られる
	db, err = sql.Open("sqlite3", "./devices.db")
	if err != nil {
		log.Fatalf("データベースを開けませんでした: %v", err)
	}
	defer db.Close()

	// テーブルの作成
	createTableSQL := `CREATE TABLE IF NOT EXISTS devices (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL,
		"mac" TEXT NOT NULL UNIQUE
	);`
	if _, err = db.Exec(createTableSQL); err != nil {
		log.Fatalf("テーブルを作成できませんでした: %v", err)
	}
	log.Println("データベースの準備ができました。")

	// Ginルーターのセットアップ

    router := gin.Default()

	// APIエンドポイント
	api := router.Group("/api")
	{
		api.GET("/devices", getDevices)
		api.POST("/devices", addDevice)
		api.PUT("/devices/:id", updateDevice)
		api.DELETE("/devices/:id", deleteDevice)
		api.POST("/wakeup/:id", wakeDevice)
	}

	// フロントエンドの静的ファイルを提供
	// embedしたファイルシステムから 'index.html' を含むサブツリーを取得
	staticFiles, _ := fs.Sub(embeddedFiles, ".")
	router.StaticFS("/", http.FS(staticFiles))


	// サーバーの起動
	log.Printf("サーバー起動: http://localhost:%s\n", appPort)
	if err := router.Run(":" + appPort); err != nil {
		log.Fatalf("サーバーの起動に失敗しました: %v", err)
	}
}

// getDevices はすべてのデバイスを取得
func getDevices(c *gin.Context) {

rows, err := db.Query("SELECT id, name, mac FROM devices ORDER BY id")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Mac); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		devices = append(devices, d)
	}
	c.JSON(http.StatusOK, devices)
}

// addDevice は新しいデバイスを追加
func addDevice(c *gin.Context) {
	var newDevice Device
	if err := c.BindJSON(&newDevice); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "入力データが不正です"})
		return
	}

	// MACアドレスのバリデーション
	if !validateMAC(newDevice.Mac) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MACアドレスの形式が正しくありません (例: 00:11:22:33:44:55)"})
		return
	}

	stmt, err := db.Prepare("INSERT INTO devices(name, mac) VALUES(?, ?)")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res, err := stmt.Exec(newDevice.Name, newDevice.Mac)
	if err != nil {
		// SQLiteのユニーク制約違反をチェック
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.JSON(http.StatusConflict, gin.H{"error": "このMACアドレスは既に登録されています"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	id, _ := res.LastInsertId()
	newDevice.ID = id
	c.JSON(http.StatusCreated, newDevice)
}

// updateDevice は既存のデバイスを更新
func updateDevice(c *gin.Context) {
	id := c.Param("id")
	var device Device
	if err := c.BindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "入力データが不正です"})
		return
	}

	// MACアドレスのバリデーション
	if !validateMAC(device.Mac) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MACアドレスの形式が正しくありません"})
		return
	}

	stmt, err := db.Prepare("UPDATE devices SET name = ?, mac = ? WHERE id = ?")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, err = stmt.Exec(device.Name, device.Mac, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.JSON(http.StatusConflict, gin.H{"error": "このMACアドレスは既に登録されています"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	idInt, _ := strconv.ParseInt(id, 10, 64)
	device.ID = idInt
	c.JSON(http.StatusOK, device)
}

// deleteDevice はデバイスを削除
func deleteDevice(c *gin.Context) {
	id := c.Param("id")
	stmt, err := db.Prepare("DELETE FROM devices WHERE id = ?")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, err = stmt.Exec(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Device deleted"})
}

// wakeDevice はWoLパケットを送信
func wakeDevice(c *gin.Context) {
	id := c.Param("id")
	var macAddr string
	err := db.QueryRow("SELECT mac FROM devices WHERE id = ?", id).Scan(&macAddr)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	
	err = sendMagicPacket(macAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Magic packet sent to %s", macAddr)})
}

// sendMagicPacket はWoLマジックパケットを生成して送信
func sendMagicPacket(macAddr string) error {
	macAddr = strings.ReplaceAll(macAddr, ":", "")
	macAddr = strings.ReplaceAll(macAddr, "-", "")
	macBytes, err := hex.DecodeString(macAddr)
	if err != nil || len(macBytes) != 6 {
		return fmt.Errorf("invalid MAC address format: %s", macAddr)
	}

	// マジックパケットのペイロードを作成 (6バイトの0xFF + 16回のMACアドレス)
	packet := make([]byte, 6, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, macBytes...)
	}

	// ブロードキャストアドレスにUDPで送信
	conn, err := net.Dial("udp", "255.255.255.255:9")
	if err != nil {
		// net.Dialが失敗した場合、ブロードキャストをサポートしていないインターフェースかもしれない
		// 代わりにListenPacketとWriteToで試みる
		log.Println("net.Dial failed, trying with ListenPacket...")
		pc, err := net.ListenPacket("udp4", ":0") // ローカルの任意のポートから送信
		if err != nil {
			return fmt.Errorf("ListenPacket failed: %w", err)
		}
		defer pc.Close()

		bcastAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: 9}
		_, err = pc.WriteTo(packet, bcastAddr)
		if err != nil {
			return fmt.Errorf("pc.WriteTo failed: %w", err)
		}

	} else {
		defer conn.Close()
		_, err = conn.Write(packet)
		if err != nil {
			return fmt.Errorf("conn.Write failed: %w", err)
		}
	}
	
	log.Printf("Sent magic packet to %s", macAddr)
	return nil
}

// validateMAC はMACアドレスの形式をチェック
func validateMAC(mac string) bool {
	// 12桁の16進数、またはコロン/ハイフン区切りの形式を許容
	regex := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	return regex.MatchString(mac)
}
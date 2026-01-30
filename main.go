package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Device represents a machine that can be woken up
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	MAC  string `json:"mac"`
}

var (
	devices     []Device
	devicesFile = "/app/data/devices.json"
	mutex       = &sync.Mutex{}
)

func main() {
	// Adjust file path for local development
	if _, err := os.Stat("/app/data"); os.IsNotExist(err) {
		devicesFile = "data/devices.json"
	}

	loadDevices()

	http.HandleFunc("/", serveTemplate)
	http.HandleFunc("/api/devices", devicesHandler)
	http.HandleFunc("/api/devices/", deviceHandler)
	http.HandleFunc("/api/wol", wolHandler)

	log.Println("Starting server on :8090")
	if err := http.ListenAndServe(":8090", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func serveTemplate(w http.ResponseWriter, r *http.Request) {
	// Adjust file path for local development
	templatePath := "index.html"
	if _, err := os.Stat("templates"); os.IsNotExist(err) {
		templatePath = "/app/index.html"
	}
	http.ServeFile(w, r, templatePath)
}

// --- Device Handlers ---

func devicesHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(devices)
	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		device.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
		devices = append(devices, device)
		saveDevices()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(device)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func deviceHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	id := r.URL.Path[len("/api/devices/"):]
	index := -1
	for i, d := range devices {
		if d.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var updatedDevice Device
		if err := json.NewDecoder(r.Body).Decode(&updatedDevice); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updatedDevice.ID = id // Ensure ID remains the same
		devices[index] = updatedDevice
		saveDevices()
		json.NewEncoder(w).Encode(updatedDevice)
	case http.MethodDelete:
		devices = append(devices[:index], devices[index+1:]...)
		saveDevices()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- WoL Handler ---

func wolHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		MAC string `json:"mac"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := sendMagicPacket(req.MAC); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send magic packet: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Magic packet sent to %s", req.MAC)
}

// --- Helper Functions ---

func loadDevices() {
	mutex.Lock()
	defer mutex.Unlock()
	file, err := ioutil.ReadFile(devicesFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("'%s' not found, starting with empty device list.", devicesFile)
			devices = []Device{}
			return
		}
		log.Fatalf("Failed to read devices file: %v", err)
	}
	if err := json.Unmarshal(file, &devices); err != nil {
		log.Fatalf("Failed to parse devices file: %v", err)
	}
	log.Printf("Loaded %d devices from '%s'", len(devices), devicesFile)
}

func saveDevices() {
	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		log.Printf("Error marshalling devices: %v", err)
		return
	}
	if err := ioutil.WriteFile(devicesFile, data, 0644); err != nil {
		log.Printf("Error writing devices file: %v", err)
	}
}

func sendMagicPacket(macAddr string) error {
	// Validate MAC address
	macPattern := `^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`
	if match, _ := regexp.MatchString(macPattern, macAddr); !match {
		return fmt.Errorf("invalid MAC address format: %s", macAddr)
	}
	
	macBytes, err := net.ParseMAC(macAddr)
	if err != nil {
		return err
	}

	// Build the magic packet
	packet := make([]byte, 102)
	// First 6 bytes of 0xFF
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	// Repeat MAC address 16 times
	for i := 1; i <= 16; i++ {
		copy(packet[i*6:(i+1)*6], macBytes)
	}

	// Broadcast the packet
	conn, err := net.Dial("udp", "255.255.255.255:9")
	if err != nil {
		// Try another broadcast address for different network configurations
		conn, err = net.Dial("udp", "192.168.1.255:9") // Common subnet, adjust if needed
		if err != nil {
			return fmt.Errorf("failed to connect for UDP broadcast: %v", err)
		}
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	return err
}
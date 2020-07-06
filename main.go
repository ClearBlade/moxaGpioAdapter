package main

import (
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	adapter_library "github.com/clearblade/adapter-go-library"
	mqttTypes "github.com/clearblade/mqtt_parsing"
)

// #cgo CFLAGS: -I./json-c/build
// #cgo LDFLAGS: -L./json-c/build -L/usr/lib/arm-linux-gnueabihf -ljson-c -lmx_gpio_ctl
// #include "mx_dio.h"
import "C"

const (
	adapterName = "moxaGpioAdapter"
	//msgSubscribeQOS = 0
	//msgPublishQOS   = 0
	//defaultTopicRoot       = "moxa-gpio"
	//requestMQTTTopic       = "<topic_root>/request/<gpio_id>"
	//changeMQTTTopic        = "<topic_root>/change/<gpio_id>"
	javascriptISOString = "2006-01-02T15:04:05.000Z07:00"
	gpioOn              = 1
	gpioOff             = 0
)

var (
	adapterConfig    *adapter_library.AdapterConfig
	currentValues    gpioValues
	readOnlyGPIOIDs  = []string{"din0", "din1", "din2", "din3"}
	readWriteGPIOIDs = []string{"dout0", "dout1", "dout2", "dout3"}
)

type gpio struct {
	GpioID           string
	Type             string
	Position         int
	Value            int
	ActiveCycle      bool
	OnInterval       int
	OffInterval      int
	CycleQuitChannel chan int
}

type gpioValues struct {
	Mutex  *sync.Mutex
	Values map[string]*gpio
}

type requestStruct struct {
	Type        string `json:"type"`
	Value       int    `json:"value"`
	OnInterval  int    `json:"on_interval"`
	OffInterval int    `json:"off_interval"`
}

type changeStruct struct {
	OldValue  int    `json:"old_value"`
	NewValue  int    `json:"new_value"`
	Timestamp string `json:"timestamp"`
}

func main() {

	// add any adapter specific command line flags needed here, before calling ParseArguments

	err := adapter_library.ParseArguments(adapterName)
	if err != nil {
		log.Fatalf("[FATAL] Failed to parse arguments: %s\n", err.Error())
	}

	// initialize all things ClearBlade, includes authenticating if needed, and fetching the
	// relevant adapter_config collection entry
	adapterConfig, err = adapter_library.Initialize()
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize: %s\n", err.Error())
	}

	// if your adapter config includes custom adapter settings, parse/validate them here
	log.Println("[DEBUG] MOXA GPIO ADAPTER INIT")
	C.mx_dio_init()
	log.Println("[DEBUG] MOXA GPIO ADAPTER INIT COMPLETE")
	//INIT is necessary before subscribe mqtt because messages may come in before IO is initilized

	// connect MQTT, if your adapter needs to subscribe to a topic, provide it as the first
	// parameter, and a callback for when messages are received. if no need to subscribe,
	// simply provide an empty string and nil
	//err = adapter_library.ConnectMQTT(adapterConfig.TopicRoot+"/request/#", cbMessageHandler)
	err = adapter_library.ConnectMQTT("moxa-gpio/request/+", cbMessageHandler)
	if err != nil {
		log.Fatalf("[FATAL] Failed to Connect MQTT: %s\n", err.Error())
	}

	// kick off adapter specific things here

	// var CrossingGPIO = {
	// 	"DigitalOutputs": {
	// 		"CROSSING_CONTROLLER": "dout0",
	// 		"FAULT_LED": "dout1",
	// 		"KRESSHAULER_SIGN": "dout2",
	// 		"MANUAL_CROSSING_RESET": "dout3"
	// 	},
	// 	"DigitalInputs": {
	// 		"BATTERY_CHARGER": "din0",
	// 		"CROSSING_ACTIVE_RELAY": "din1",
	// 		"MANUAL_OVERRIDE": "din2"
	// 		//"NOT_USED": "din3"
	// 	}
	// };

	// var MobileGPIO = {
	// 	"DigitalOutputs": {
	// 		"GPS_LOCK_LED": "dout0",
	// 		"LORA_JOIN_LED": "dout1",
	// 		"CROSSING_ACTIVE_LED": "dout2",
	// 		"SYSTEM_BYPASSED_LED": "dout3"
	// 	},
	// 	"DigitalInputs": {
	// 		"SYSTEM_BYPASS_SWITCH": "din0",
	// 		"MANUAL_CROSSING_BUTTON": "din1"
	// 	}
	// };

	log.Println("[DEBUG] initGPIOValues - loading initial values")

	currentValues = gpioValues{
		Mutex:  &sync.Mutex{},
		Values: make(map[string]*gpio),
	}

	for _, gpioID := range readOnlyGPIOIDs {
		re := regexp.MustCompile("[0-9]+")
		var pos = re.FindAllString(gpioID, -1)[0]
		val, err := strconv.Atoi(pos)
		if err != nil {
			log.Printf("[ERROR] Failed to convert value to int for gpio position %s: %s\n", pos, err.Error())
		}
		thisGPIO := &gpio{
			GpioID:   gpioID,
			Type:     "din",
			Position: val,
		}
		thisGPIO.readValueFromMoxa()
		currentValues.Mutex.Lock()
		currentValues.Values[gpioID] = thisGPIO
		currentValues.Mutex.Unlock()
		log.Printf("[DEBUG] initGPIOValues - initial value for %s is : %d\n", thisGPIO.GpioID, thisGPIO.Value)
	}
	for _, gpioID := range readWriteGPIOIDs {
		re := regexp.MustCompile("[0-9]+")
		var pos = re.FindAllString(gpioID, -1)[0]
		val, err := strconv.Atoi(pos)
		if err != nil {
			log.Printf("[ERROR] Failed to convert value to int for gpio position %s: %s\n", pos, err.Error())
		}
		thisGPIO := &gpio{
			GpioID:   gpioID,
			Type:     "dout",
			Position: val,
		}
		thisGPIO.readValueFromMoxa()
		currentValues.Mutex.Lock()
		currentValues.Values[gpioID] = thisGPIO
		currentValues.Mutex.Unlock()
		log.Printf("[DEBUG] initGPIOValues - initial value for %s is : %d\n", thisGPIO.GpioID, thisGPIO.Value)
	}

	for x := 0; x < len(readOnlyGPIOIDs); x++ {
		go startPolling(readOnlyGPIOIDs[x])
	}

	// keep adapter executing indefinitely
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Printf("[INFO] Listening for GPIO changes or write requests...")
		}
	}
}

func cbMessageHandler(message *mqttTypes.Publish) {

	log.Printf("[DEBUG] cbMessageHandler - received raw message %s on topic %s\n", string(message.Payload), message.Topic.Whole)
	if len(message.Topic.Split) == 3 && message.Topic.Split[1] == "request" {
		var requestMsg requestStruct
		if err := json.Unmarshal(message.Payload, &requestMsg); err != nil {
			log.Printf("[ERROR] cbMessageHandler - failed to unmarshal incoming mqtt request message: %s\n", err.Error())
			return
		}
		gpioID := message.Topic.Split[len(message.Topic.Split)-1]
		currentValues.Mutex.Lock()
		var theGPIO *gpio
		var ok = true
		if theGPIO, ok = currentValues.Values[gpioID]; !ok {
			currentValues.Mutex.Unlock()
			log.Printf("[ERROR] cbMessageHandler - unexpected gpio id: %s\n", gpioID)
			return
		}
		currentValues.Mutex.Unlock()
		switch requestMsg.Type {
		case "write":
			log.Printf("[DEBUG] cbMessageHandler - processing gpio write request: %+v\n", requestMsg)
			theGPIO.writeNewValueToFile(requestMsg.Value)
			break
		case "start_cycle":
			log.Printf("[DEBUG] cbMessageHandler - processing gpio start_cycle request: %+v\n", requestMsg)
			if theGPIO.ActiveCycle {
				log.Printf("[ERROR]")
				break
			}
			theGPIO.startGPIOCycle(requestMsg.OnInterval, requestMsg.OffInterval)
			break
		case "end_cycle":
			log.Printf("[DEBUG] cbMessageHandler - processing gpio end_cycle request: %+v\n", requestMsg)
			theGPIO.stopGPIOCycle(requestMsg.Value)
			break
		default:
			log.Printf("[ERROR] cbMessageHandler - unexpected request type: %s\n", requestMsg.Type)
			break
		}
	}

}

func (g *gpio) readValueFromMoxa() {
	//log.Printf("[DEBUG] readValueFromMoxa - reading value for gpio id: %s\n", g.GpioID)
	var state = 2
	cstate := C.int(state)
	cpos := C.int(g.Position)
	if g.Type == "din" {
		C.mx_din_get_state(cpos, &cstate)
	} else {
		C.mx_dout_get_state(cpos, &cstate)
	}

	if cstate < 0 {
		log.Printf("[ERROR] readValueFromMoxa - read value %d from gpio id: %s\n", cstate, g.GpioID)
		return
	}
	g.Value = int(cstate)
	//log.Printf("[DEBUG] readValueFromMoxa - read value %d from gpio id: %s\n", g.Value, g.GpioID)
}

func startPolling(gpioID string) {
	timestamp := time.Now().Format(javascriptISOString)
	ticker := time.NewTicker(200 * time.Millisecond)
	for t := range ticker.C {
		currentValues.Mutex.Lock()
		changedGPIO := currentValues.Values[gpioID]
		currentValues.Mutex.Unlock()
		oldValue := changedGPIO.Value
		changedGPIO.readValueFromMoxa() //ERROR HANDLING ??
		newValue := changedGPIO.Value
		if oldValue != newValue {
			log.Println("[INFO] startPolling - got a new value tick at ", t)
			log.Printf("[INFO] new value for %s is %d\n", gpioID, newValue)
			publishGPIOChange(gpioID, oldValue, newValue, timestamp)
		} else {
			//log.Println("[DEBUG] startPolling - value didn't actually change, not doing anything at tick ", t)
		}
	}
	defer ticker.Stop()
}

func publishGPIOChange(gpioID string, oldValue, newValue int, timestamp string) {
	log.Println("[DEBUG] publishGPIOChange - received gpio change to publish")
	//"<topic_root>/change/<gpio_id>"
	topic := "moxa-gpio/change/<gpio_id>"
	topic = strings.Replace(topic, "<gpio_id>", gpioID, 1)
	changeMessage := &changeStruct{
		OldValue:  oldValue,
		NewValue:  newValue,
		Timestamp: timestamp,
	}
	msgBytes, err := json.Marshal(changeMessage)
	if err != nil {
		log.Printf("[ERROR] publishGPIOChange - failed to marshal change message: %s\n", err.Error())
	}
	if err := adapter_library.Publish(topic, msgBytes); err != nil {
		log.Printf("[ERROR] publishGPIOChange - failed to publish change :%s\n", err.Error())
		return
	}
	log.Println("[DEBUG] publishGPIOChange - Change successfully published")
}

func (g *gpio) writeNewValueToFile(newValue int) {
	log.Printf("[DEBUG] writeNewValueToFile - writing value %d for gpio id: %s\n", newValue, g.GpioID)
	if g.Value == newValue {
		log.Println("[DEBUG] writeNewValueToFile - provided new value is same as current, doing nothing")
		return
	}
	cnewValue := C.int(newValue)
	cpos := C.int(g.Position)
	ret := C.mx_dout_set_state(cpos, cnewValue)
	if ret < 0 {
		log.Printf("[ERROR] failed to write value to file for gpio %s: %d\n", g.GpioID, ret)
		return
	}
	g.Value = newValue
	log.Printf("[INFO] writeNewValueToFile - successfully wrote new value for gpio id: %s\n", g.GpioID)
}

func (g *gpio) startGPIOCycle(onInterval, offInterval int) {
	log.Printf("[DEBUG] startGPIOCycle - starting gpio cycle for gpio id: %s\n", g.GpioID)
	g.OnInterval = onInterval
	g.OffInterval = offInterval
	g.CycleQuitChannel = make(chan int)
	g.ActiveCycle = true
	go g.cycleGPIO()
	log.Printf("[INFO] startGPIOCycle - successully started gpio cycle for gpio id: %s\n", g.GpioID)
}

func (g *gpio) stopGPIOCycle(newValue int) {
	log.Printf("[DEBUG] stopGPIOCycle - stopping gpio cycle for gpio id: %s\n", g.GpioID)
	if g.ActiveCycle {
		g.CycleQuitChannel <- newValue
	} else {
		log.Println("[ERROR] stopGPIOCycle - gpio is not currently cycling")
	}
	log.Printf("[INFO] stopGPIOCycle - successully stopped gpio cycle for gpio id: %s\n", g.GpioID)
}

func (g *gpio) cycleGPIO() {
	totalInterval := g.OnInterval + g.OffInterval
	g.writeNewValueToFile(gpioOn)
	onTicker := time.NewTicker(time.Duration(totalInterval) * time.Second)
	defer onTicker.Stop()
	time.Sleep(time.Duration(g.OnInterval) * time.Second)
	offTicker := time.NewTicker(time.Duration(totalInterval) * time.Second)
	defer offTicker.Stop()
	for {
		select {
		case newValue := <-g.CycleQuitChannel:
			g.writeNewValueToFile(newValue)
			g.ActiveCycle = false
			g.CycleQuitChannel = nil
			return
		case <-onTicker.C:
			g.writeNewValueToFile(gpioOff)
			break
		case <-offTicker.C:
			g.writeNewValueToFile(gpioOn)
			break
		}
	}
}

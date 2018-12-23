package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// examples:
// {"time" : "2018-12-22 18:11:07", "model" : "Acurite Rain Gauge", "id" : 136, "rain" : 1030.500}
// {"time" : "2018-12-22 18:11:07", "brand" : "OS", "model" : "OSv1 Temperature Sensor", "sid" : 1, "channel" : 1, "battery" : "OK", "temperature_C" : 8.400}
// {"time" : "2018-12-22 18:11:07", "model" : "Acurite Rain Gauge", "id" : 136, "rain" : 1030.500}
// {"time" : "2018-12-22 18:11:18", "model" : "inFactory sensor", "id" : 147, "temperature_F" : 47.300, "humidity" : 1}
// Inovalley kw9015b", "id" : 105, "temperature_C" : 9.900, "rain" : 232}
type WeatherSensor struct {
	Time          string
	Model         string
	Brand         string
	Sid           int
	Id            int
	Channel       int
	Battery       string
	Temperature_C float64
	Rain          float64
	Temperature_F float64
	Humidity      int
}

//define a function for the default message handler
var publishHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())
}

func main() {
	hostname, _ := os.Hostname()
	clientId := "rtl433_2_" + hostname
	publishUri := "rtl433/" + hostname

	//create a ClientOptions struct setting the broker address, clientid, turn
	//off trace output and set the default message handler
	opts := MQTT.NewClientOptions().AddBroker("tcp://mqttserver.internal:1883")
	opts.SetClientID(clientId)
	opts.SetDefaultPublishHandler(publishHandler)

	for true {
		output, err := runRTL433()
		if err != nil {
			fmt.Println(err)
		}

		//create and start a client using the above ClientOptions
		c := MQTT.NewClient(opts)
		if token := c.Connect(); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
		}

		// Read rtl433 output
		var sensorReading WeatherSensor
		/*file, err := os.Open("rtl433.out")
		if err != nil {
			panic(err)
		}
		defer file.Close()*/

		//scanner := bufio.NewScanner(file)
		scanner := bufio.NewScanner(strings.NewReader(output))
		for scanner.Scan() {
			line := scanner.Text()
			//fmt.Println(line)
			// Save JSON to Post struct
			if err := json.Unmarshal([]byte(line), &sensorReading); err != nil {
				//fmt.Println(err)
			}

			sensorReading.Model = StripSpaces(sensorReading.Model)
			//fmt.Println(sensorReading.Model)

			if sensorReading.Model == "AcuriteRainGauge" {
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_rain", sensorReading.Rain)
			} else if sensorReading.Model == "OSv1TemperatureSensor" {
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Sid)+"_"+strconv.Itoa(sensorReading.Channel)+"_temp", math.Round(sensorReading.Temperature_C*10)/10)
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Sid)+"_"+strconv.Itoa(sensorReading.Channel)+"_batt", sensorReading.Battery)
			} else if sensorReading.Model == "inFactorysensor" {
				tempCelsius := (sensorReading.Temperature_F - 32) / 1.8
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_temp", math.Round(tempCelsius*10)/10)
			} else if strings.HasPrefix(sensorReading.Model, "Inovalley") {
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_rain", sensorReading.Rain)
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_temp", math.Round(sensorReading.Temperature_C*10)/10)
			} else if sensorReading.Model == "AlectoV1TemperatureSensor" {
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_temp", math.Round(sensorReading.Temperature_C*10)/10)
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_batt", sensorReading.Battery)
			} else if strings.HasPrefix(sensorReading.Model, "TFA") {
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_"+strconv.Itoa(sensorReading.Channel)+"_temp", math.Round(sensorReading.Temperature_C*10)/10)
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_"+strconv.Itoa(sensorReading.Channel)+"_batt", sensorReading.Battery)
				sendMQTT(c, publishUri+"/"+sensorReading.Model+"_"+strconv.Itoa(sensorReading.Id)+"_"+strconv.Itoa(sensorReading.Channel)+"_humid", sensorReading.Humidity)
			}
		}

		c.Disconnect(250)
	}
}

func dealwithErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func sendMQTT(client MQTT.Client, uri string, message interface{}) {
	var token MQTT.Token
	switch m := message.(type) {
	case string:
		token = client.Publish(uri, 0, false, m)
	case float64, float32, []float64, []float32:
		token = client.Publish(uri, 0, false, fmt.Sprintf("%.1f", m))
	case uint, uint64, uint32, int, int32, int64:
		token = client.Publish(uri, 0, false, fmt.Sprintf("%d", m))
	}
	token.Wait()
}

func StripSpaces(str string) string {
	var b strings.Builder
	b.Grow(len(str))
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// helper method for executing commands on OS
func executeCommand(cmd string, args ...string) (string, error) {
	var out []byte
	command := exec.Command(cmd, args...)
	out, err := command.CombinedOutput()

	return string(out), err
}

// runRTL433 runs rtl_433 for 15 seconds
func runRTL433() (string, error) {
	output, err := executeCommand("rtl_433", "-G", "-Fjson", "-T 60")
	fmt.Println(output)
	return output, err
}

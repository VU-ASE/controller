package main

import (
	"fmt"
	"os"
	"time"

	pb_outputs "github.com/VU-ASE/rovercom/v2/packages/go/outputs"
	roverlib "github.com/VU-ASE/roverlib-go/v2/src"
	"github.com/rs/zerolog/log"
	pid "go.einride.tech/pid"
)

const (
	ERROR_SCALER = 100.0
)

// Global values for OTA tuning
var pidController pid.Controller
var speed float64

// Compare float values for equality
func floatEqual(a, b float64) bool {
	return a-b < 0.0001 && b-a < 0.0001
}

func initializePIDController(kp, ki, kd float64) pid.Controller {
	result := pid.Controller{
		Config: pid.ControllerConfig{
			ProportionalGain: float64(kp),
			IntegralGain:     float64(ki),
			DerivativeGain:   float64(kd),
		},
	}
	return result
}

func calculateSteerValue(pidController pid.Controller, trajectoryPoints []*pb_outputs.HorizontalScan, resolution *pb_outputs.Resolution) float64 {
	// Ideally, the center of the horizontal scan, should be at the center of the image
	desired := float64(resolution.GetWidth()) / 2.0
	// This is the center of the horizontal scan
	actual := float64(trajectoryPoints[0].XRight+trajectoryPoints[0].XLeft) / 2.0

	// Use the PID controller to decide where to go
	pidController.Update(pid.ControllerInput{
		// We use the ERROR_SCALER so that the Kp, Ki and Kd values do not become too small
		// (see the service.yaml file for the default values)
		ReferenceSignal:  desired / ERROR_SCALER,
		ActualSignal:     actual / ERROR_SCALER,
		SamplingInterval: 10 * time.Second,
	})
	steerValue := pidController.State.ControlSignal
	// min-max
	if steerValue > 1 {
		steerValue = 1
	} else if steerValue < -1 {
		steerValue = -1
	}
	return -steerValue // has to be inverted
}

func sendOutput(actuatorOutput *roverlib.WriteStream, speed float64, steerValue float64) error {
	err := actuatorOutput.Write(
		&pb_outputs.SensorOutput{
			SensorId:  2,
			Timestamp: uint64(time.Now().UnixMilli()),
			SensorOutput: &pb_outputs.SensorOutput_ControllerOutput{
				ControllerOutput: &pb_outputs.ControllerOutput{
					SteeringAngle: float32(steerValue),
					LeftThrottle:  float32(speed),
					RightThrottle: float32(speed),
					FrontLights:   false,
				},
			},
		},
	)
	return err
}

func run(
	service roverlib.Service, config *roverlib.ServiceConfiguration) error {

	//
	// Set up stream to read track from
	//
	imagingInput := service.GetReadStream("imaging", "path")

	//
	// Set up stream to write actuator data to
	//
	actuatorOutput := service.GetWriteStream("decision")

	//
	// Get configuration values
	//

	if config == nil {
		return fmt.Errorf("No configuration provided")
	}

	// Get PID tuning values
	kp, err := config.GetFloatSafe("kp")
	if err != nil {
		return err
	}
	ki, err := config.GetFloatSafe("ki")
	if err != nil {
		return err
	}
	kd, err := config.GetFloatSafe("kd")
	if err != nil {
		return err
	}
	speed, err = config.GetFloatSafe("speed")
	if err != nil {
		return err
	}

	pidController = initializePIDController(kp, ki, kd)

	// Main loop, subscribe to trajectory data and send decision data
	for {
		log.Debug().Msg("looping")
		data, err := imagingInput.Read()
		if err != nil {
			return err
		}

		// Parse imaging data
		imagingData := data.GetCameraOutput()
		if imagingData == nil {
			log.Warn().Msg("Received sensor data that was not camera data")
			continue
		}

		// Get the horizontal scan data
		horizontal_scans := imagingData.GetHorizontalScans()
		if horizontal_scans == nil {
			log.Warn().Msg("Received sensor data that was not horizontal scan data")
			continue
		}

		speed, err = config.GetFloat("speed")
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get speed from config")
		}

		newKp, perr := config.GetFloat("kp")
		newKd, derr := config.GetFloat("kd")
		newKi, ierr := config.GetFloat("ki")
		if perr == nil && derr == nil && ierr == nil && (!floatEqual(newKp, kp) || !floatEqual(newKd, kd) || !floatEqual(newKi, ki)) {
			kp = newKp
			kd = newKd
			ki = newKi

			pidController = initializePIDController(kp, ki, kd)

			log.Info().Float64("kp", kp).Float64("kd", kd).Float64("ki", ki).Msg("Updated PID controller")
		}

		// Get the first horizontal scan
		if len(horizontal_scans) == 0 {
			log.Warn().Msg("Received sensor data that had no horizontal scans")
			continue
		}
		resolution := imagingData.GetResolution()
		if resolution == nil {
			log.Warn().Msg("Received sensor data that had no resolution")
			continue
		}

		steerValue := calculateSteerValue(pidController, horizontal_scans, resolution)

		err = sendOutput(actuatorOutput, speed, steerValue)

		// Send it for the actuator (and others) to use
		if err != nil {
			log.Err(err).Msg("Failed to send controller output")
			continue
		}

		log.Debug().Msg("Sent controller output")
	}
}

func onTerminate(sig os.Signal) error {
	return nil
}

// Used to start the program with the correct arguments
func main() {
	roverlib.Run(run, onTerminate)
}

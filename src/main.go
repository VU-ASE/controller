package main

import (
	"fmt"
	"os"
	"time"

	pb_outputs "github.com/VU-ASE/rovercom/packages/go/outputs"
	roverlib "github.com/VU-ASE/roverlib-go/src"
	"github.com/rs/zerolog/log"
	pid "go.einride.tech/pid"
)

// Global values for OTA tuning
var pidController pid.Controller
var speed float64

func run(
	service roverlib.Service, config *roverlib.ServiceConfiguration) error {

	//
	// Set up stream to read track from
	//
	imagingInput := service.GetReadStream("imaging", "path")

	// 
	// Set up stream to read distance
	// 
	distanceInput := service.GetReadStream("distance", "distance-m")

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
	desiredTrajectoryPointFloat, err := config.GetFloatSafe("desired-trajectory-point")
	if err != nil {
		return err
	}
	desiredTrajectoryPoint := int(desiredTrajectoryPointFloat)

	// Initialize pid controller
	pidController = pid.Controller{
		Config: pid.ControllerConfig{
			ProportionalGain: float64(kp),
			IntegralGain:     float64(ki),
			DerivativeGain:   float64(kd),
		},
	}

	// Main loop, subscribe to trajectory data and send decision data
	for {
		log.Debug().Msg("looping")


		data, err := distanceInput.Read()
		if err != nil {
			return err
		}

		distanceMeters := data.GetDistanceOutput().GetDistance()

		log.Info().Float32("fucking distance", distanceMeters).Msg("ffs")

		if distanceMeters <= 0.25 {
			err = actuatorOutput.Write(
				&pb_outputs.SensorOutput{
					SensorId:  2,
					Timestamp: uint64(time.Now().UnixMilli()),
					SensorOutput: &pb_outputs.SensorOutput_ControllerOutput{
						ControllerOutput: &pb_outputs.ControllerOutput{
							SteeringAngle: float32(0),
							LeftThrottle:  float32(0),
							RightThrottle: float32(0),
							FrontLights:   false,
						},
					},
				},
			)
			// Send it for the actuator (and others) to use
			if err != nil {
				log.Err(err).Msg("Failed to send controller output")
			}
			continue
		}


		data, err = imagingInput.Read()
		if err != nil {
			return err
		}

		// Parse imaging data
		imagingData := data.GetCameraOutput()
		if imagingData == nil {
			log.Warn().Msg("Received sensor data that was not camera data")
			continue
		}

		// Get trajectory
		trajectory := imagingData.GetTrajectory()
		if trajectory == nil {
			log.Warn().Msg("Received sensor data that was not trajectory data")
			continue
		}

		speed, err = config.GetFloat("speed")
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get speed from config")
		}

		newKp, perr := config.GetFloat("kp")
		newKd, derr := config.GetFloat("kd")
		newKi, ierr := config.GetFloat("ki")
		if perr == nil && derr == nil && ierr == nil && (newKp != kp || newKd != kd || newKi != ki) {
			kp = newKp
			kd = newKd
			ki = newKi

			// Create a new PID controller with the new values
			pidController = pid.Controller{
				Config: pid.ControllerConfig{
					ProportionalGain: float64(kp),
					IntegralGain:     float64(ki),
					DerivativeGain:   float64(kd),
				},
			}

			log.Info().Float64("kp", kp).Float64("kd", kd).Float64("ki", ki).Msg("Updated PID controller")
		}

		// Get the first trajectory point
		trajectoryPoints := trajectory.GetPoints()
		if len(trajectoryPoints) == 0 {
			log.Warn().Msg("Received sensor data that had no trajectory points")
			continue
		}
		// This is the middle of the longest consecutive slice, it should be in the middle of the image (horizontally)
		firstPoint := trajectoryPoints[0]

		// Use the PID controller to decide where to go
		pidController.Update(pid.ControllerInput{
			ReferenceSignal:  float64(desiredTrajectoryPoint),
			ActualSignal:     float64(firstPoint.X),
			SamplingInterval: 100 * time.Millisecond,
		})
		steerValue := pidController.State.ControlSignal
		log.Info().Float64("steerValue", steerValue).Int("Desired", desiredTrajectoryPoint).Float32("Actual", float32(firstPoint.X)).Msg("Calculated steering value")
		// min-max
		if steerValue > 1 {
			steerValue = 1
		} else if steerValue < -1 {
			steerValue = -1
		}
		steerValue = -steerValue

		err = actuatorOutput.Write(
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

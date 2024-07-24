package main

import (
	"os"
	"time"

	pb_outputs "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/outputs"
	pb_systemmanager_messages "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/systemmanager"
	servicerunner "github.com/VU-ASE/pkg-ServiceRunner/v2/src"
	zmq "github.com/pebbe/zmq4"
	"github.com/rs/zerolog/log"
	pid "go.einride.tech/pid"
	"google.golang.org/protobuf/proto"
)

// Global values for OTA tuning
var pidController pid.Controller
var speed float32

func run(
	service servicerunner.ResolvedService,
	sysMan servicerunner.SystemManagerInfo,
	initialTuning *pb_systemmanager_messages.TuningState) error {

	// Get the address of trajectory data output by the imaging module
	imagingTrajectoryAddress, err := service.GetDependencyAddress("imaging", "path")
	if err != nil {
		return err
	}

	// Get the address to which to send the decision data for the actuator to use
	decisionAddress, err := service.GetOutputAddress("decision")
	if err != nil {
		return err
	}

	// Create a socket to send decision data on
	outputSock, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		return err
	}
	err = outputSock.Bind(decisionAddress)
	if err != nil {
		return err
	}

	// Create a socket to receive trajectory data on
	imagingSock, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		return err
	}
	err = imagingSock.Connect(imagingTrajectoryAddress)
	if err != nil {
		return err
	}
	err = imagingSock.SetSubscribe("") // Subscribe to all messages
	if err != nil {
		return err
	}

	// Get PID tuning values
	kp, err := servicerunner.GetTuningFloat("kp", initialTuning)
	if err != nil {
		return err
	}
	ki, err := servicerunner.GetTuningFloat("ki", initialTuning)
	if err != nil {
		return err
	}
	kd, err := servicerunner.GetTuningFloat("kd", initialTuning)
	if err != nil {
		return err
	}

	// Get speed to use
	speed, err = servicerunner.GetTuningFloat("speed", initialTuning)
	if err != nil {
		return err
	}

	// Get the desired trajectory point
	desiredTrajectoryPoint, err := servicerunner.GetTuningInt("desired-trajectory-point", initialTuning)
	if err != nil {
		return err
	}

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
		// Receive trajectory data
		sensorBytes, err := imagingSock.RecvBytes(0)
		if err != nil {
			return err
		}

		log.Debug().Msg("Received imaging data")

		// Parse as protobuf message
		sensorOutput := &pb_outputs.SensorOutput{}
		err = proto.Unmarshal(sensorBytes, sensorOutput)
		if err != nil {
			log.Err(err).Msg("Failed to unmarshal trajectory data")
			continue
		}

		// Parse imaging data
		imagingData := sensorOutput.GetCameraOutput()
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

		// Get the first trajectory point
		trajectoryPoints := trajectory.GetPoints()
		if len(trajectoryPoints) == 0 {
			log.Warn().Msg("Received sensor data that had no trajectory points")
			continue
		}
		firstPoint := trajectoryPoints[0]
		// This is the middle of the longest consecutive slice, it should be in the middle of the image (horizontally)

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

		// Create controller output, wrapped in generic sensor output
		controllerOutput := &pb_outputs.SensorOutput{
			SensorId:  1,
			Timestamp: uint64(time.Now().UnixMilli()),
			SensorOutput: &pb_outputs.SensorOutput_ControllerOutput{
				ControllerOutput: &pb_outputs.ControllerOutput{
					SteeringAngle: float32(steerValue),
					LeftThrottle:  speed,
					RightThrottle: speed,
					FrontLights:   false,
				},
			},
		}

		// Marshal the controller output
		controllerBytes, err := proto.Marshal(controllerOutput)
		if err != nil {
			log.Err(err).Msg("Failed to marshal controller output")
			continue
		}

		// Send it for the actuator (and others) to use
		_, err = outputSock.SendBytes(controllerBytes, 0)
		if err != nil {
			log.Err(err).Msg("Failed to send controller output")
			continue
		}

		log.Debug().Msg("Sent controller output")
	}
}

func onTuningState(newtuning *pb_systemmanager_messages.TuningState) {
	log.Warn().Msg("Tuning state changed")
	// Get speed to use
	newSpeed, err := servicerunner.GetTuningFloat("speed", newtuning)
	if err != nil {
		log.Err(err).Msg("Failed to get new speed")
		return
	}
	speed = newSpeed

	// Create a new PID controller
	kp, err := servicerunner.GetTuningFloat("kp", newtuning)
	if err != nil {
		log.Err(err).Msg("Failed to get new kp")
		return
	}
	ki, err := servicerunner.GetTuningFloat("ki", newtuning)
	if err != nil {
		log.Err(err).Msg("Failed to get new ki")
		return
	}
	kd, err := servicerunner.GetTuningFloat("kd", newtuning)
	if err != nil {
		log.Err(err).Msg("Failed to get new kd")
		return
	}

	// Initialize pid controller
	pidController = pid.Controller{
		Config: pid.ControllerConfig{
			ProportionalGain: float64(kp),
			IntegralGain:     float64(ki),
			DerivativeGain:   float64(kd),
		},
	}

}

func onTerminate(sig os.Signal) {
	log.Info().Msg("Terminating")
}

// Used to start the program with the correct arguments
func main() {
	servicerunner.Run(run, onTuningState, onTerminate, false)
}

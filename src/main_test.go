package main

import (
	"testing"
	"fmt"
	// roverlib "github.com/VU-ASE/roverlib-go/src"
	pb_outputs "github.com/VU-ASE/rovercom/packages/go/outputs"
)

// func TestInitializePIDController(t *testing.T) {
// 	kp := 0.003
// 	kd := 0.00001
// 	ki := 0
// }

func TestCalculateSteerValue(t *testing.T) {
	kp := 0.003
	kd := 0.00001
	ki := float64(0)

	controller := initializePIDController(kp, ki, kd)

	desiredPoint := 320

	tests := []struct {
		trajectoryPoints 	[]*pb_outputs.CameraSensorOutput_Trajectory_Point
		want 				float64
	}{
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 300, Y: 120},
			},
			want: -0.062,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 320, Y: 120},
			},
			want: 0,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 319, Y: 120},
			},
			want: -0.0031,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 321, Y: 120},
			},
			want: 0.0031,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 0, Y: 120},
			},
			want: -0.992,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 640, Y: 120},
			},
			want: 0.992,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: -100, Y: 120},
			},
			want: -1,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 800, Y: 120},
			},
			want: 1,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 160, Y: 120},
			},
			want: -0.496,
		},
		{
			trajectoryPoints: []*pb_outputs.CameraSensorOutput_Trajectory_Point {
				{X: 480, Y: 120},
			},
			want: 0.496,
		},
	}

	for _, tt := range tests {

		testname := fmt.Sprintf("actual X: %d, desired X: %d", tt.trajectoryPoints[0].X, desiredPoint)
		t.Run(testname, func(t *testing.T) {
			have := calculateSteerValue(controller, tt.trajectoryPoints, desiredPoint)
			if tt.want != have {
				t.Errorf("calculateSteerValue() = %f, want: %f", have, tt.want)
			}
		})
	}
}

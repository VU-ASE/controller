# Processing

This service follows the following steps:

1. It reads a [`CameraSensorOutput`](https://github.com/VU-ASE/rovercom/blob/main/definitions/outputs/camera.proto) message from the `path` stream as defined in the [*service.yaml* file](https://github.com/VU-ASE/controller/blob/f9a5d84299681e87bd6da4757e56af634ce32142/service.yaml#L15)
2. This message should contain one or more [`HorizontalScan` objects](https://github.com/VU-ASE/rovercom/blob/c1d6569558e26d323fecc17d01117dbd089609cc/definitions/outputs/camera.proto#L42). These horizontal scans represent the edges of the track (so `xLeft` is the left edge, and `xRight` is the right edge)
3. The `xLeft` and `xRight` positions are normalized between 0 and 100.0 based on the [`resolution`](https://github.com/VU-ASE/rovercom/blob/c1d6569558e26d323fecc17d01117dbd089609cc/definitions/outputs/camera.proto#L37) reported by the `imaging` service
4. Then, the mid point (between `xLeft` and `xRight`) is taken. This should be at 50.0 when the Rover is on the middle of the track. The difference between the desired value (50.0) and the actual value is called the *error*. The error is scaled and passed to a PID controller to decide how much to steer the front wheels, and in which direction
5. Finally, the steering value and acceleration values are encoded in the [`ControllerOutput` message](https://github.com/VU-ASE/rovercom/blob/c1d6569558e26d323fecc17d01117dbd089609cc/definitions/outputs/controller.proto#L12), which is written to the [`decision` stream](https://github.com/VU-ASE/controller/blob/f9a5d84299681e87bd6da4757e56af634ce32142/service.yaml#L18) 

## What is a PID Controller?

PID stands for Proportional Integral Derivative, and in context of this application, it aims to remain as close as possible to the calculated center of the track. Before making the decision which way to turn, the controller takes into consideration all 3 terms with equal weight and acts based on the result. Below is a short description of each of the terms:

- **Proportional** - considers the current error, i.e. the distance between the observed center of the track and where the car currently is.
- **Integral** - takes the cummulative error over time, not just the current one like the **Proportional** term. It would try to steer the car towards the center also if it observes a small error present over a longer period of time, whereas **Proportional** would not make a significant enough adjustment. 
- **Derivative** - works by taking the difference between the measured errors and divides it by the change in time. This allows it to see how fast the error is changing. While **Proportional** term takes into account the current value, **Integral** - past, **Derivative** tries to predict the future errors.

Each of the terms can, of course, be used as a standalone controller by itself, but it is unlikely to yield a better result. You can find a more detailed description of the mechanism [here](https://www.integrasources.com/blog/basics-of-pid-controllers-design-applications/#:~:text=A%20PID%20controller%20calculates%20the,the%20whole%20term%20becomes%20zero.), or a very detailed description [here](https://en.wikipedia.org/wiki/Proportional–integral–derivative_controller).
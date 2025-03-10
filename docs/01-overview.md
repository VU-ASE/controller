# Overview
The `controller` service takes pre-processed track edge data and transforms it into actuator decision commands based on a simple PID controller.

### A PID controller?

PID stands for Proportional Integral Derivative, and in context of this application, it aims to remain as close as possible to the calculated center of the track. Before making the decision which way to turn, the controller takes into consideration all 3 terms with equal weight and acts based on the result. Below is a short description of each of the terms:

**Proportional** - considers the current error, i.e. the distance between the observed center of the track and where the car currently is.

**Integral** - takes the cummulative error over time, not just the current one like the **Proportional** term. It would try to steer the car towards the center also if it observes a small error present over a longer period of time, whereas **Proportional** would not make a significant enough adjustment. 

**Derivative** - works by taking the difference between the measured errors and divides it by the change in time. This allows it to see how fast the error is changing. While **Proportional** term takes into account the current value, **Integral** - past, **Derivative** tries to predict the future errors.

Each of the terms can, of course, be used as a standalone controller by itself, but it is unlikely to yield a better result. You can find a more detailed description of the mechanism [here](https://www.integrasources.com/blog/basics-of-pid-controllers-design-applications/#:~:text=A%20PID%20controller%20calculates%20the,the%20whole%20term%20becomes%20zero.), or a very detailed description [here](https://en.wikipedia.org/wiki/Proportional–integral–derivative_controller).
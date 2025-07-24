# Controly - Load Test

This directory contains the load testing script for the Controly server. It is designed to systematically measure the server's performance and determine the maximum number of concurrent connections it can sustain under a specified load.

## Test Specification

### Core Concept

The load test operates in incremental stages, referred to as "Levels." Each level introduces a new batch of simulated Controller and Display clients to the server. The test runs for a configurable duration at each level, during which it actively monitors server health and client stability. If any performance metric exceeds its predefined threshold (e.g., excessive latency, high resource consumption, or client disconnections), the test is terminated. The final report will indicate the highest level the server successfully passed.

A "connection" is defined as the sum of all active Display and Controller WebSocket clients connected to the server.

### Client Simulation

- **Controller**: Each simulated Controller connects to the server, subscribes to a specified number of Displays, and sends `ping` commands at a configurable rate (`cpm`). It listens for `status` updates from Displays to calculate command-response latency.
- **Display**: Each simulated Display connects to the server using a `command.json` file hosted by a local static server. It listens for `ping` commands and immediately responds by sending a `status` update containing a `pong` payload. It also sends its own status updates at a configurable rate (`spm`).

### Performance & Failure Metrics

The test monitors the following key metrics to determine server health:

1.  **Client Stability**: Any unexpected disconnection or error from a client's WebSocket will immediately fail the current level.
2.  **Command Latency**: The round-trip time for a `ping` command from a Controller to a Display and back. If the average latency exceeds the configured maximum, the test fails.
3.  **Server Resources**: The script periodically polls a `/api/metrics` endpoint on the server to fetch:
    - CPU Usage (%)
    - Memory Usage (MB)
    If either resource exceeds its configured maximum, the test fails.

## How to Run the Test

1.  **Start the Controly Server**: In a separate terminal, navigate to the `server` directory and start the server.

    ```bash
    cd ../server
    go run main.go
    ```

2.  **Install Dependencies**: Navigate to the `load-test` directory and install the necessary dependencies.

    ```bash
    cd ../load-test
    pnpm install
    ```

3.  **Execute the Load Test**: Run the test using the `pnpm start` command. The script will automatically start a local static server for `command.json` and then begin the test.

    ```bash
    pnpm start
    ```

### Command-Line Arguments

The test script can be configured via the following command-line arguments.

| Argument                    | Alias | Description                                           | Default Value                  |
| --------------------------- | ----- | ----------------------------------------------------- | ------------------------------ |
| `--dpc`                     |       | Number of Display clients per Controller              | `1`                            |
| `--cpm`                     |       | Commands sent per minute by each Controller           | `60`                           |
| `--spm`                     |       | Status updates sent per minute by each Display        | `60`                           |
| `--serverUrl`               |       | WebSocket URL of the Controly server                  | `ws://localhost:8080/ws`       |
| `--metricsUrl`              |       | HTTP URL for the server metrics endpoint              | `http://localhost:8080/api/metrics` |
| `--stepSize`                |       | Number of Controllers to add at each test level       | `10`                           |
| `--duration`                |       | Duration (in seconds) for each test level             | `60`                           |
| `--maxLatency`              |       | Maximum acceptable average command latency (ms)       | `500`                          |
| `--maxCpu`                  |       | Maximum acceptable average server CPU usage (%)       | `90`                           |
| `--maxMemory`               |       | Maximum acceptable average server memory usage (MB)   | `1024`                         |

**Example:** To run the test with 2 displays per controller and a step size of 5:

```bash
pnpm start -- --dpc 2 --stepSize 5
```

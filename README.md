# picam
A simple server for taking pictures and videos on the raspberry-pi over http.

Requires MP4Box installation on the pi:

    sudo apt-get install -y gpac

## Arguements

    Usage of ./picam:
    -p int
            port to bind to (default 8000)

## Routes

### `GET` /pic.jpg
Capture a still image with the camera, returned as a .jpg in the response body.

### `POST` /video/start
Start a video recording with the camera, returning nothing in the response body.

### `GET` /video/stop
Stops the video recording, returning the video as an mp4 in the response body.

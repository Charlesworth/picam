package camera

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Picam interface {
	StartRecording() error
	StopRecording() (filename string, err error)
	Capture() (picture []byte, err error)
}

type picam struct {
	videoCtx          context.Context
	videoCtxCancel    context.CancelFunc
	recordingStopChan chan error

	cameraState *cameraState
}

const (
	errorTakingPicture   string = "camera in use, taking picture"
	errorRecordingVideo  string = "camera in use, recording video"
	errorProcessingVideo string = "camera in use, processing a finished video"
	errorNotRecording    string = "camera was not recording, unable to process stop recording request"
	errorUnexpectedState string = "unexpected camera state %v"
	errorStopRecording   string = "unexpected error closing camera: %v"
)

func NewPicam() Picam {
	return &picam{
		cameraState: newCameraState(),
	}
}

func (p *picam) StartRecording() error {
	err := p.cameraState.toState(stateVideoRecording)
	if err != nil {
		return err
	}

	os.Remove("vid.h264")

	ctx, cancelFunc := context.WithCancel(context.Background())
	p.videoCtx = ctx
	p.videoCtxCancel = cancelFunc
	p.recordingStopChan = make(chan error)

	go func() {
		cmd := exec.CommandContext(ctx, "raspivid", "-o", "vid.h264", "-t", "10000000")
		_, err := cmd.Output()
		p.recordingStopChan <- err
	}()

	// sleep for half a second to allow the camera to wake
	time.Sleep(time.Millisecond * 20)

	// check for init errors
	select {
	case initError := <-p.recordingStopChan:
		p.cameraState.toState(stateFree)
		return initError
	default:
		return nil
	}
}

func (p *picam) StopRecording() (filename string, err error) {
	err = p.cameraState.toState(stateVideoStopping)
	if err != nil {
		return "", err
	}
	defer p.cameraState.toState(stateFree)

	// cancel the record command and give time for the video file to be closed
	p.videoCtxCancel()
	time.Sleep(time.Millisecond * 20)

	select {
	case err := <-p.recordingStopChan:
		if err.Error() != "signal: killed" {
			return "", fmt.Errorf(errorStopRecording, err.Error())
		}
	}

	os.Remove("vid.mp4")

	// convert the video to mp4 format
	cmd := exec.Command("MP4Box", "-add", "vid.h264", "vid.mp4")
	_, err = cmd.Output()
	if err != nil {
		return "", err
	}

	return "vid.mp4", nil
}

func (p *picam) Capture() (picture []byte, err error) {
	err = p.cameraState.toState(statePictureTaking)
	if err != nil {
		return nil, err
	}
	defer p.cameraState.toState(stateFree)

	cmd := exec.Command("raspistill", "-o", "-")
	return cmd.Output()
}

type cameraStateEnum int

const (
	stateFree           cameraStateEnum = 0
	stateVideoRecording cameraStateEnum = 1
	stateVideoStopping  cameraStateEnum = 2
	statePictureTaking  cameraStateEnum = 3
)

type cameraState struct {
	currentState cameraStateEnum
	mut          sync.Mutex
}

func newCameraState() *cameraState {
	return &cameraState{
		currentState: stateFree,
		mut:          sync.Mutex{},
	}
}

func (cs *cameraState) toState(desiredState cameraStateEnum) error {
	cs.mut.Lock()
	defer cs.mut.Unlock()
	// operations freeing the state should always work
	if desiredState == stateFree {
		cs.currentState = stateFree
	}

	switch cs.currentState {
	// if the camera is free allow capture or start recording but not stop recording
	case stateFree:
		if desiredState == stateVideoStopping {
			return errors.New(errorNotRecording)
		}
		cs.currentState = desiredState
		return nil

	// if the camera is taking a picture, block all operations
	case statePictureTaking:
		return errors.New(errorTakingPicture)

	// if the camera is recording, only allow video stopping operation
	case stateVideoRecording:
		if desiredState == stateVideoStopping {
			cs.currentState = desiredState
			return nil
		}
		return errors.New(errorRecordingVideo)

	// if the camera is stopping and processing a video, block all operations
	case stateVideoStopping:
		return errors.New(errorProcessingVideo)
	}

	// any unhandled states should throw an error
	return fmt.Errorf(errorUnexpectedState, desiredState)
}

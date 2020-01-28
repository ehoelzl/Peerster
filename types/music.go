package types

import (
	"time"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"math/rand"
	"math"
	"sync"
)

const amplitude float64 = 0.1
const kickFreq float64 = 22
const hiHatFreq float64 = 1174.66
const bassAmplitude float64 = 0.6
const synthAmplitude float64 = 0.4
const sampleRate float64 = 44100

var drumPlaying = false
var drumLock = &sync.Mutex{}

var sr = InitSpeaker(sampleRate)

func Bass(freq float64) beep.Streamer {
	var inc float64 = 0
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			samples[i][0] = bassAmplitude * math.Abs(math.Sin((2 * math.Pi * inc * freq) / sampleRate))
			samples[i][1] = bassAmplitude * math.Abs(math.Sin((2 * math.Pi * inc * freq) / sampleRate))
			inc++
		}
		return len(samples), true
	})
}

func Synth(freq1 float64, freq2 float64, freq3 float64) beep.Streamer {
	var inc float64 = 0
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			samples[i][0] = 0.33 * synthAmplitude * (math.Sin((2 * math.Pi * inc * freq1) / sampleRate)) +
							0.33 * synthAmplitude * (math.Sin((2 * math.Pi * inc * freq2) / sampleRate)) +
							0.33 * synthAmplitude * (math.Sin((2 * math.Pi * inc * freq3) / sampleRate))
			samples[i][1] = 0.33 * synthAmplitude * (math.Sin((2 * math.Pi * inc * freq1) / sampleRate)) +
							0.33 * synthAmplitude * (math.Sin((2 * math.Pi * inc * freq2) / sampleRate)) +
							0.33 * synthAmplitude * (math.Sin((2 * math.Pi * inc * freq3) / sampleRate))
			inc++
		}
		return len(samples), true
	})
}

func Kick() beep.Streamer {
	var inc float64 = 1
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			amplitude := 0.4
			value := math.Abs(math.Sin((2 * math.Pi * inc * kickFreq) / sampleRate))
			// shape := math.Sqrt(0.1 / math.Pow((inc / sampleRate), 2))
			shape := 1 - (4 * inc / sampleRate)
			samples[i][0] = (value * 2 - 1) * shape * amplitude
			samples[i][1] = (value * 2 - 1) * shape * amplitude
			inc++
		}
		return len(samples), true
	})
}

func Snare() beep.Streamer {
	var inc float64 = 1
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			value := rand.Float64()
			shape := 1.0 // math.Sqrt(1 / math.Pow((inc / sampleRate), 2))
			shape = 1 - (2 * inc / sampleRate)
			samples[i][0] = (value * 2 - 1) * shape * amplitude
			samples[i][1] = (value * 2 - 1) * shape * amplitude
			inc++
		}
		return len(samples), true
	})
}

func HiHat() beep.Streamer {
	var inc float64 = 1
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			shape := 1.0 // math.Sqrt(1 / math.Pow((inc / sampleRate), 2))
			shape = 1 - (20 * inc / sampleRate)
			samples[i][0] = amplitude * shape * (rand.Float64() * 2 - 1)
			samples[i][1] = amplitude * shape * (rand.Float64() * 2 - 1)
			inc++
		}
		return len(samples), true
	})
}

func InitSpeaker(sampleRate float64) beep.SampleRate {
	sr := beep.SampleRate(sampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
	return sr
}

func DrumLoop(sr beep.SampleRate) beep.Streamer {
	return beep.Seq(
		beep.Take(sr.N(250*time.Millisecond), Kick()),
		beep.Take(sr.N(100*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(300*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(250*time.Millisecond), Snare()),
		beep.Take(sr.N(100*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(125*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(175*time.Millisecond), Kick()),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(125*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(175*time.Millisecond), Kick()),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(300*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(250*time.Millisecond), Snare()),
		beep.Take(sr.N(100*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(300*time.Millisecond), beep.Silence(-1)))
}

func BassLoop(sr beep.SampleRate) beep.Streamer {
	return beep.Seq(
		beep.Take(sr.N(300*time.Millisecond), Bass(73.42)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(73.42)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(82.41)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(87.31)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(98.00)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(110.00)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(87.31)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(82.41)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(73.42)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(55.00)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(73.42)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(87.31)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(98.00)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(300*time.Millisecond), Bass(87.31)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(650*time.Millisecond), Bass(65.41)),
		beep.Take(sr.N(50*time.Millisecond), beep.Silence(-1)))

}

func SynthLoop(sr beep.SampleRate) beep.Streamer {
	return beep.Seq(
		beep.Take(sr.N(1300*time.Millisecond), Synth(392.00, 493.88, 587.33)),
		beep.Silence(sr.N(100*time.Millisecond)),
		beep.Take(sr.N(1300*time.Millisecond), Synth(392.00, 523.25, 587.33)),
		beep.Silence(sr.N(100*time.Millisecond)),
		beep.Take(sr.N(1300*time.Millisecond), Synth(349.23, 440.00, 587.33)),
		beep.Silence(sr.N(100*time.Millisecond)),
		beep.Take(sr.N(1300*time.Millisecond), Synth(349.23, 440.00, 587.33)))
}

func PlayDrums() {
	drumLock.Lock()
	if drumPlaying {
		drumLock.Unlock()
		return
	}
	drumPlaying = true
	drumLock.Unlock()

	done := make(chan bool)

	drumStream := beep.Seq(
		DrumLoop(sr),
		DrumLoop(sr),
		DrumLoop(sr),
		DrumLoop(sr),
		DrumLoop(sr),
		DrumLoop(sr),
		DrumLoop(sr),
		DrumLoop(sr))

    speaker.Play(beep.Seq(
		drumStream,
		beep.Callback(func() { done <- true })))

	<-done
	drumLock.Lock()
	drumPlaying = false
	drumLock.Unlock()
}

func PlayBass() {
	done := make(chan bool)

	bassStream := beep.Seq(
		BassLoop(sr),
		BassLoop(sr))

    speaker.Play(beep.Seq(
		bassStream,
		beep.Callback(func() { done <- true })))

	<-done
}

func PlaySynth() {
	done := make(chan bool)

    synthStream := beep.Seq(
		SynthLoop(sr),
		SynthLoop(sr))

    speaker.Play(beep.Seq(
		synthStream,
		beep.Callback(func() { done <- true })))

	<-done
}

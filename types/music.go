package types

import (
	"time"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"math/rand"
	"math"
	"sync"
)

const kickFreq float64 = 22
const hiHatFreq float64 = 1174.66
const bassAmplitude float64 = 0.1
const synthAmplitude float64 = 0.4
const sampleRate float64 = 44100

var drumPlaying = false
var drumLock = &sync.Mutex{}

var sr = InitSpeaker(sampleRate)

func LowPass(cutoff, resonance float64) (a1, a2, a3, b1, b2 float64) {
	// c = 1.0f / (float)Math.Tan(Math.PI * frequency / sampleRate);
	c := 1.0 / math.Tan(math.Pi * cutoff / sampleRate)
	// a1 = 1.0f / (1.0f + resonance * c + c * c);
	a1 = 1.0 / (1.0 + resonance * c + c * c)
	// a2 = 2f * a1;
	a2 = 2.0 * a1
	// a3 = a1;
	a3 = a1
	// b1 = 2.0f * (1.0f - c * c) * a1;
	b1 = 2.0 * (1.0 - c * c) * a1
	// b2 = (1.0f - resonance * c + c * c) * a1;
	b2 = (1.0 - resonance * c + c * c) * a1
	return a1, a2, a3, b1, b2
}

func Bass(freq float64) beep.Streamer {
	var inc float64 = 0
	attack := 0.01
	sustain := 0.22
	release := 0.1
	resonance := 0.3
	cutoff := 2800.0

	a1, a2, a3, b1, b2 := LowPass(cutoff, resonance)

	v_1 := 0.0
	v_2 := 0.0
	o_1 := 0.0
	o_2 := 0.0

	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {

			var amp float64
			if inc < sampleRate * attack {
				amp = inc * 1 / (attack * sampleRate)
			} else if inc < sampleRate * (sustain + attack) {
				amp = 1.0
			} else if inc < sampleRate * (sustain + attack + release) {
				amp = (inc - sampleRate * (sustain + attack)) * -1 / (release * sampleRate) + 1
			} else {
				amp = 0
			}

			// value := math.Abs(math.Sin((2 * math.Pi * inc * freq) / sampleRate))
			value := math.Mod(((2 * inc * freq) / sampleRate), 2) - 1

			v := (value * amp * bassAmplitude)
			s_curr := a1 * v + a2 * v_1 + a3 * v_2 - b1 * o_1 - b2 * o_2

			samples[i][0] = s_curr
			samples[i][1] = s_curr

			v_2 = v_1
			v_1 = v
			o_2 = o_1
			o_1 = s_curr

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
	attack := 0.0
	sustain := 0.1
	release := 0.05
	cutoff := 250.0
	resonance := 0.3

	a1, a2, a3, b1, b2 := LowPass(cutoff, resonance)

	v_1 := 0.0
	v_2 := 0.0
	o_1 := 0.0
	o_2 := 0.0

	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {

			var amp float64
			if inc < sampleRate * attack {
				amp = inc * 1 / (attack * sampleRate)
			} else if inc < sampleRate * (sustain + attack) {
				amp = 1.0
			} else if inc < sampleRate * (sustain + attack + release) {
				amp = (inc - sampleRate * (sustain + attack)) * -1 / (release * sampleRate) + 1
			} else {
				amp = 0
			}

			amplitude := 0.5
			value := math.Abs(math.Sin((2 * math.Pi * inc * kickFreq) / sampleRate))

			v := (value * 2 - 1) * amplitude * amp

			// COMPLEX METHOD
			// float newOutput = a1 * newInput + a2 * this.inputHistory[0] + a3 * this.inputHistory[1] - b1 * this.outputHistory[0] - b2 * this.outputHistory[1];
			s_curr := a1 * v + a2 * v_1 + a3 * v_2 - b1 * o_1 - b2 * o_2

			samples[i][0] = s_curr
			samples[i][1] = s_curr

			v_2 = v_1
			v_1 = v
			o_2 = o_1
			o_1 = s_curr

			inc++
		}
		return len(samples), true
	})
}

func Snare() beep.Streamer {
	var inc float64 = 1
	amplitude := 0.2
	cutoff := 3800.0
	resonance := 0.8
	
	a1, a2, a3, b1, b2 := LowPass(cutoff, resonance)
	
	v_1 := 0.0
	v_2 := 0.0
	o_1 := 0.0
	o_2 := 0.0


	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			value := rand.Float64()
			shape := 1.0 // math.Sqrt(1 / math.Pow((inc / sampleRate), 2))
			shape = 1 - (3 * inc / sampleRate)

			// COMPLEX METHOD
			// float newOutput = a1 * newInput + a2 * this.inputHistory[0] + a3 * this.inputHistory[1] - b1 * this.outputHistory[0] - b2 * this.outputHistory[1];
			v := (value * 2 - 1) * shape * amplitude

			s_curr := a1 * v + a2 * v_1 + a3 * v_2 - b1 * o_1 - b2 * o_2

			samples[i][0] = s_curr
			samples[i][1] = s_curr

			v_2 = v_1
			v_1 = v
			o_2 = o_1
			o_1 = s_curr

			inc++
		}
		return len(samples), true
	})
}

func HiHat() beep.Streamer {
	var inc float64 = 1
	amplitude := 0.2
	cutoff := 2200.0
	resonance := 0.1

	a1, a2, a3, b1, b2 := LowPass(cutoff, resonance)

	v_1 := 0.0
	v_2 := 0.0
	o_1 := 0.0
	o_2 := 0.0

	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			shape := 1.0 // math.Sqrt(1 / math.Pow((inc / sampleRate), 2))
			shape = 1 - (20 * inc / sampleRate)

			v := amplitude * shape * (rand.Float64() * 2 - 1)

			// COMPLEX METHOD
			// float newOutput = a1 * newInput + a2 * this.inputHistory[0] + a3 * this.inputHistory[1] - b1 * this.outputHistory[0] - b2 * this.outputHistory[1];
			s_curr := a1 * v + a2 * v_1 + a3 * v_2 - b1 * o_1 - b2 * o_2

			samples[i][0] = s_curr
			samples[i][1] = s_curr

			v_2 = v_1
			v_1 = v
			o_2 = o_1
			o_1 = s_curr

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
		beep.Mix(
			beep.Take(sr.N(250*time.Millisecond), Kick()),
			beep.Take(sr.N(50*time.Millisecond), HiHat())),
		beep.Take(sr.N(100*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(300*time.Millisecond), beep.Silence(-1)),
		beep.Mix(
			beep.Take(sr.N(250*time.Millisecond), Snare()),
			beep.Take(sr.N(50*time.Millisecond), HiHat())),
		beep.Take(sr.N(100*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(125*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(175*time.Millisecond), Kick()),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(125*time.Millisecond), beep.Silence(-1)),
		beep.Take(sr.N(175*time.Millisecond), Kick()),
		beep.Take(sr.N(50*time.Millisecond), HiHat()),
		beep.Take(sr.N(300*time.Millisecond), beep.Silence(-1)),
		beep.Mix(
			beep.Take(sr.N(250*time.Millisecond), Snare()),
			beep.Take(sr.N(50*time.Millisecond), HiHat())),
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

	speaker.Clear()

	drumPlaying = true
	drumLock.Unlock()

	done := make(chan bool)

	drumStream := beep.Seq(
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
	speaker.Clear()

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
	speaker.Clear()

	done := make(chan bool)

    synthStream := beep.Seq(
		SynthLoop(sr),
		SynthLoop(sr))

    speaker.Play(beep.Seq(
		synthStream,
		beep.Callback(func() { done <- true })))

	<-done
}

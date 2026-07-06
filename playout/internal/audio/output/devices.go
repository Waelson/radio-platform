package output

// DeviceInfo descreve um dispositivo de saída de áudio disponível no sistema.
//
// O significado do campo ID varia por driver:
//   - coreaudio: UID persistente do sistema (kAudioDevicePropertyDeviceUID),
//     mantido mesmo que o nome do dispositivo seja alterado.
//   - portaudio: igual a Name — PortAudio não expõe UID interno; se o
//     dispositivo for renomeado no SO, o ID muda junto.
//   - null / file: valor fixo ("null" ou "file").
type DeviceInfo struct {
	ID                string  // identificador único (semântica varia por driver — ver acima)
	Name              string  // nome legível (ex: "MacBook Pro Speakers")
	Driver            string  // "coreaudio" | "portaudio" | "null" | "file"
	HostAPI           string  // "ALSA" | "PulseAudio" | "JACK" | "CoreAudio" | "WASAPI" | ""
	IsDefault         bool    // true se for o output padrão do sistema
	MaxOutputChannels int     // número máximo de canais de saída suportados
	DefaultSampleRate float64 // taxa de amostragem padrão reportada pelo SO
}

// DeviceLister é implementado por qualquer OutputDevice capaz de enumerar
// os dispositivos de saída de áudio disponíveis no sistema sem precisar
// de um stream aberto.
type DeviceLister interface {
	ListDevices() ([]DeviceInfo, error)
}

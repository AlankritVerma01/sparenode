package doctor

import "testing"

func TestParseNvidiaSMI(t *testing.T) {
	input := "0, NVIDIA GeForce GTX 1650, GPU-abc, 4096, 610.57.04"
	gpus, err := ParseNvidiaSMI(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(gpus) != 1 || gpus[0].Name != "NVIDIA GeForce GTX 1650" || gpus[0].MemoryMiB != 4096 {
		t.Fatalf("unexpected GPUs: %#v", gpus)
	}
}

func TestParseNvidiaSMIRejectsMalformedRows(t *testing.T) {
	if _, err := ParseNvidiaSMI("0, incomplete"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseDF(t *testing.T) {
	input := "Filesystem 1-blocks Used Available Capacity Mounted on\n/dev/sda1 1000 250 750 25% /data"
	storage, err := parseDF("/data", input)
	if err != nil {
		t.Fatal(err)
	}
	if storage.SizeBytes != 1000 || storage.AvailableBytes != 750 {
		t.Fatalf("unexpected storage: %#v", storage)
	}
}

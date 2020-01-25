package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

func generator(jobs chan job, files []string, height, width int, input, output string) {
	generators.Add(1)
	defer generators.Done()
	for _, file := range files {
		jobs <- job{
			file:   file,
			height: height,
			width:  width,
			input:  input,
			output: output,
		}
	}
}

func worker(jobs chan job, done chan jobDone) {
	workers.Add(1)
	defer workers.Done()
	for {
		work, keepWorking := <-jobs
		if keepWorking {
			file, err := os.Open(work.input + work.file)
			if err == nil {
				image, _, err := image.Decode(file)
				if err == nil {
					img := resize.Resize(
						uint(work.width),
						uint(work.height),
						image,
						resize.Lanczos3,
					)
					done <- jobDone{
						file:  work.output + work.file,
						image: img,
					}
				}
				file.Close()
			}

		} else {
			log.Printf("all jobs have been processed")
			return
		}
	}
}

func merger(done chan jobDone) {
	mergers.Add(1)
	defer mergers.Done()
	i := 1
	for job := range done {
		log.Printf(
			"(%.2f %%) [%d/%d] processing %s",
			float64(i*100)/float64(total),
			i,
			total,
			job.file,
		)
		i++
		out, err := os.Create(job.file)
		if err == nil {
			jpeg.Encode(out, job.image, &jpeg.Options{Quality: 100})
			out.Close()
		}

	}
}

type job struct {
	file   string
	input  string
	output string
	height int
	width  int
}

type jobDone struct {
	file  string
	image image.Image
}

var mergers sync.WaitGroup
var workers sync.WaitGroup
var generators sync.WaitGroup
var images = map[string]map[string]string{}
var total = 0

func main() {
	flag.Usage = func() {
		fmt.Println("How to run:\n\timageScaler [-flags]")
		flag.PrintDefaults()
	}
	extension := flag.String("extension", ".jpg", "file extension")
	input := flag.String("input", "", "folder containing input images")
	output := flag.String("output", "", "folder where output images where be placed")
	height := flag.Int("height", 64, "height of the scaled image")
	width := flag.Int("width", 64, "width of the scaled image")

	flag.Parse()
	if *input == "" || *output == "" {
		flag.Usage()
		return
	}
	nWorker := 6
	jobs := make(chan job, nWorker*2)
	done := make(chan jobDone, nWorker*2)

	files, err := ioutil.ReadDir(*input)
	if err != nil {
		log.Fatal(err)
	}

	validFiles := []string{}
	for _, f := range files {
		if !f.IsDir() {
			if strings.HasSuffix(f.Name(), *extension) {
				validFiles = append(validFiles, f.Name())
			}
		}
	}
	total = len(validFiles)
	go generator(jobs, validFiles, *height, *width, *input, *output)
	go merger(done)
	for n := 1; n <= nWorker; n++ {
		log.Printf("starting worker %d", n)
		go worker(jobs, done)
	}
	var sleep time.Duration = 3
	generators.Wait()
	log.Printf("all generators for folder '%s' are done... waiting %d seconds before continuing...", *input, sleep)
	time.Sleep(sleep * time.Second)
	close(jobs)
	workers.Wait()
	log.Printf("all workers for folder '%s' are done... waiting %d seconds before continuing...", *input, sleep)
	time.Sleep(sleep * time.Second)
	close(done)
	mergers.Wait()
	log.Printf("all mergers for folder '%s' are done... waiting %d seconds before continuing...", *input, sleep)
	time.Sleep(sleep * time.Second)
	log.Printf("done...")
}

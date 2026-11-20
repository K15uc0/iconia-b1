// iconia-relay: túnel TCP "reverso" usando apenas `adb forward`.
//
// Android < 5.0 não tem `adb reverse`, então um app no tablet (cliente VNC)
// não consegue alcançar 127.0.0.1 do PC pelo cabo USB. Este programa resolve
// isso com dois lados:
//
//	device: roda no tablet. Escuta o cliente VNC em 127.0.0.1:5900 e escuta o
//	        PC em 127.0.0.1:5901 (alcançado via `adb forward`).
//	host:   roda no PC. Mantém conexões ociosas abertas até o tablet; quando o
//	        tablet sinaliza um cliente novo, conecta no servidor VNC local e
//	        une os dois fluxos.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

var version = "dev"

// signalByte é enviado do device para o host quando um cliente chega.
const signalByte = 0x01

func main() {
	log.SetFlags(log.Ltime)
	log.SetPrefix("iconia-relay ")
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "device":
		fs := flag.NewFlagSet("device", flag.ExitOnError)
		client := fs.String("client", "127.0.0.1:5900", "endereço onde o cliente VNC do tablet conecta")
		host := fs.String("host", "127.0.0.1:5901", "endereço onde o PC conecta (alvo do adb forward)")
		wait := fs.Duration("wait", 10*time.Second, "quanto esperar por uma conexão do PC")
		_ = fs.Parse(os.Args[2:])
		log.Fatal(runDevice(*client, *host, *wait, nil))
	case "host":
		fs := flag.NewFlagSet("host", flag.ExitOnError)
		device := fs.String("device", "127.0.0.1:6001", "porta local encaminhada ao tablet (adb forward)")
		vnc := fs.String("vnc", "127.0.0.1:5900", "servidor VNC local")
		idle := fs.Int("idle", 2, "conexões ociosas mantidas abertas")
		_ = fs.Parse(os.Args[2:])
		runHost(*device, *vnc, *idle, nil)
	case "version", "-version", "--version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "uso: iconia-relay device|host [flags]   (iconia-relay <modo> -h para detalhes)\n")
}

// ---------- lado do tablet ----------

func runDevice(clientAddr, hostAddr string, wait time.Duration, ready chan<- struct{}) error {
	hl, err := net.Listen("tcp", hostAddr)
	if err != nil {
		return err
	}
	cl, err := net.Listen("tcp", clientAddr)
	if err != nil {
		hl.Close()
		return err
	}
	log.Printf("device: cliente em %s, PC em %s", clientAddr, hostAddr)
	if ready != nil {
		close(ready)
	}

	idle := make(chan net.Conn, 32)
	go func() {
		for {
			h, err := hl.Accept()
			if err != nil {
				log.Printf("device: accept PC: %v", err)
				return
			}
			select {
			case idle <- h:
			default:
				h.Close() // fila cheia
			}
		}
	}()

	for {
		c, err := cl.Accept()
		if err != nil {
			return err
		}
		go func() {
			h := takeAlive(idle, wait)
			if h == nil {
				log.Printf("device: nenhum PC conectado em %s; cliente descartado", wait)
				c.Close()
				return
			}
			if _, err := h.Write([]byte{signalByte}); err != nil {
				c.Close()
				h.Close()
				return
			}
			log.Printf("device: sessão aberta (%s)", c.RemoteAddr())
			pipe(c, h)
			log.Printf("device: sessão encerrada")
		}()
	}
}

// takeAlive retira da fila a primeira conexão do PC que ainda está viva.
func takeAlive(idle <-chan net.Conn, wait time.Duration) net.Conn {
	deadline := time.After(wait)
	for {
		select {
		case h := <-idle:
			if alive(h) {
				return h
			}
			h.Close()
		case <-deadline:
			return nil
		}
	}
}

// alive detecta conexões fechadas pelo outro lado (EOF) sem consumir dados:
// o host nunca envia nada antes do sinal, então qualquer leitura bem-sucedida
// ou EOF indica que a conexão não serve.
func alive(c net.Conn) bool {
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
	var b [1]byte
	_, err := c.Read(b[:])
	_ = c.SetReadDeadline(time.Time{})
	var ne net.Error
	return err != nil && errors.As(err, &ne) && ne.Timeout()
}

// ---------- lado do PC ----------

func runHost(deviceAddr, vncAddr string, idle int, stop <-chan struct{}) {
	log.Printf("host: tablet via %s -> VNC %s (%d ociosas)", deviceAddr, vncAddr, idle)
	var wg sync.WaitGroup
	for i := 0; i < idle; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hostWorker(deviceAddr, vncAddr, stop)
		}()
	}
	wg.Wait()
}

func hostWorker(deviceAddr, vncAddr string, stop <-chan struct{}) {
	backoff := 500 * time.Millisecond
	for {
		select {
		case <-stop:
			return
		default:
		}
		d, err := net.Dial("tcp", deviceAddr)
		if err != nil {
			sleep(backoff, stop)
			continue
		}
		var b [1]byte
		if _, err := io.ReadFull(d, b[:]); err != nil || b[0] != signalByte {
			// Com adb forward o connect sempre funciona; EOF aqui significa
			// que o relay ainda não está rodando no tablet.
			d.Close()
			sleep(backoff, stop)
			continue
		}
		v, err := net.Dial("tcp", vncAddr)
		if err != nil {
			log.Printf("host: VNC local indisponível: %v", err)
			d.Close()
			sleep(backoff, stop)
			continue
		}
		log.Printf("host: sessão aberta")
		go func() {
			pipe(d, v)
			log.Printf("host: sessão encerrada")
		}()
	}
}

func sleep(d time.Duration, stop <-chan struct{}) {
	select {
	case <-time.After(d):
	case <-stop:
	}
}

// pipe copia nos dois sentidos; quando um lado termina, fecha os dois
// (servidores VNC como o x11vnc ignoram half-close).
func pipe(a, b net.Conn) {
	var once sync.Once
	closeBoth := func() { a.Close(); b.Close() }
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		once.Do(closeBoth)
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
	<-done
}

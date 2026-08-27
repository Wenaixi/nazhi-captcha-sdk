package captchasdk

import "net/http"

func fetchForBench() (string, []byte, error) {
	resp, err := http.Get(Base + "/kaptcha/kaptcha.jpg")
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	img := make([]byte, 0, 2048)
	buf := make([]byte, 512)
	for {
		n, err := resp.Body.Read(buf)
		img = append(img, buf[:n]...)
		if err != nil {
			break
		}
	}
	return "", img, nil
}

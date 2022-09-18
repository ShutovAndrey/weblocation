package provider

import (
	"archive/zip"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"github.com/ShutovAndrey/weblocation/pkg/logger"
	"github.com/pkg/errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IpAd struct {
	// ipNet *net.IPNet
	Ip   uint32
	Code string
}

func isMaxMindDB(n string) bool {
	switch n {
	case "Country", "City":
		return true
	}
	return false
}

func downloadDB(dbType string) (map[string]string, error) {
	var path, tmpDir, uri string

	if isMaxMindDB(dbType) {
		key, ok := os.LookupEnv("MAXMIND_KEY")
		if ok {
			uri = fmt.Sprintf(
				"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-%s-CSV&license_key=%s&suffix=zip",
				dbType, key)
		} else {
			if dbType == "Country" {
				uri = "https://gist.github.com/ShutovAndrey/16f98ad0cff549a782a31942e456f1ba/raw/8ca6715285b02b125772c45ae0b67babc21cad95/GeoLite2-Country-CSV.zip"
			} else {
				uri = "https://gist.github.com/ShutovAndrey/dada04a211a785856cd383c858410c8c/raw/78775d68b7ed276538bdc11811f1d15182a56169/GeoLite2-City-CSV.zip"
			}
		}
	} else if dbType == "Currency" {
		uri =
			"https://gist.githubusercontent.com/HarishChaudhari/4680482/raw/b61a5bdf5f3d5c69399f9d9e592c4896fd0dc53c/country-code-to-currency-code-mapping.csv"
	} else {
		return nil, errors.New("Unknown DB")
	}

	resp, err := http.Get(uri)
	if err != nil {
		return nil, errors.Wrap(err, "Can't download file")
	}

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("Received non 200 response code")
	}

	defer resp.Body.Close()

	var contentName string
	if dbType == "Currency" {
		contentName = "country-code-to-currency-code-mapping.csv"
	} else {
		name, ok := resp.Header["Content-Disposition"]
		if !ok {
			contentName = fmt.Sprintf("GeoLite2-%s-CSV-%s.zip", dbType, time.Now().Format("01022006"))
			logger.Info("No content-desposition header. The default name setted")
		} else {
			contentName = strings.Split(name[0], "filename=")[1]

			if len(contentName) == 0 {
				contentName = fmt.Sprintf("GeoLite2-%s-CSV-%s.zip", dbType, time.Now().Format("01022006"))
				logger.Info("empty contentName. The default name setted")
			}

		}
	}

	tmpDir = os.TempDir()
	path = filepath.Join(tmpDir, contentName)

	out, err := os.Create(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't create file %s", path)

	}
	defer out.Close()

	// Change permissions
	err = os.Chmod(path, 0665)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't change permission to file %s", path)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't copy to file %s", path)
	}

	var files map[string]string
	files = make(map[string]string)

	if dbType != "Currency" {
		var err error
		files, err = unzip(path, tmpDir, dbType)
		if err != nil {
			return nil, err
		}
	} else {
		files[dbType] = path
	}

	return files, nil
}

func unzip(path, dst, dbType string) (map[string]string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, errors.Wrap(err, "Can't open archive with files")
	}
	defer archive.Close()

	files := make(map[string]string)

	types := [2]string{"Locations-en", "Blocks-IPv4"}

	for _, f := range archive.File {

		//use only IPv4 ranges and countries'n'cities codes
		if !strings.HasSuffix(f.Name, fmt.Sprintf("GeoLite2-%s-Blocks-IPv4.csv", dbType)) &&
			!strings.HasSuffix(f.Name, fmt.Sprintf("GeoLite2-%s-Locations-en.csv", dbType)) {
			continue
		}

		filePath := filepath.Join(dst, f.Name)

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "Can't create directory %s", filePath)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, errors.Wrapf(err, "Can't open file %s", filePath)
		}
		fileInArchive, err := f.Open()
		if err != nil {
			return nil, errors.Wrapf(err, "File is broken %s", f.Name)

		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return nil, errors.Wrapf(err, "Can't copy file %s", filePath)
		}

		dstFile.Close()
		fileInArchive.Close()

		for _, t := range types {
			if strings.Contains(f.Name, t) {
				files[t] = filePath
			}
		}
	}
	return files, nil
}

func readCsvFile(filePath string, key, value uint8) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	dict := make(map[string]string, len(records))

	for i, record := range records {
		if i == 0 {
			continue
		}

		length := uint8(len(record))
		if key > length || value > length {
			return nil, errors.New("Invalid key-value pair")
		}
		dict[record[key]] = record[value]
	}
	if len(dict) != 0 {
		return dict, nil
	} else {
		return nil, errors.New("Empty map")
	}

}

func readCsvFileIP(filePath string) ([]IpAd, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	ipAdresses := make([]IpAd, 0, len(records))

	for i, record := range records {
		if i == 0 {
			continue
		}
		ip, _, err := net.ParseCIDR(record[0])
		if err != nil {
			return nil, err
		}
		ipEncoded := binary.BigEndian.Uint32(ip[12:16])
		new := IpAd{Ip: ipEncoded, Code: record[1]}

		ipAdresses = append(ipAdresses, new)

	}
	if len(ipAdresses) != 0 {
		return ipAdresses, nil
	} else {
		return nil, errors.Errorf("Empty map")
	}

}

func GetFromDB(name string) (*[]IpAd, *map[string]string) {
	fileNames, err := downloadDB(name)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info(fmt.Sprintf("database %s successfully downloaded", name))
	}

	if isMaxMindDB(name) {
		ipAdresses, err := readCsvFileIP(fileNames["Blocks-IPv4"])
		if err != nil {
			logger.Error(err)
		}

		var key, value uint8
		if name == "City" {
			key, value = 0, 10
		} else {
			key, value = 0, 4
		}

		locations, err := readCsvFile(fileNames["Locations-en"], key, value)
		if err != nil {
			logger.Error(err)
		}
		return &ipAdresses, &locations
	}

	currencies, err := readCsvFile(fileNames["Currency"], 1, 3)
	if err != nil {
		logger.Error(err)
	}

	return nil, &currencies
}

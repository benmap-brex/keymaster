package authutil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/foomo/htpasswd"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/ldap.v2"
)

func CheckHtpasswdUserPassword(username string, password string, htpasswdBytes []byte) (bool, error) {
	//	secrets := HtdigestFileProvider(htpasswdFilename)
	passwords, err := htpasswd.ParseHtpasswd(htpasswdBytes)
	if err != nil {
		return false, err
	}
	hash, ok := passwords[username]
	if !ok {
		return false, nil
	}
	// only understand bcrypt
	if !strings.HasPrefix(hash, "$2y$") {
		err := errors.New("Can only use bcrypt for htpasswd")
		return false, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return false, nil
	}
	return true, nil

}

func getLDAPConnection(u url.URL, timeoutSecs uint, rootCAs *x509.CertPool) (*ldap.Conn, string, error) {
	if u.Scheme != "ldaps" {
		err := errors.New("Invalid ldap scheme (we only support ldaps")
		return nil, "", err
	}
	//hostnamePort := server + ":636"
	serverPort := strings.Split(u.Host, ":")
	port := "636"
	if len(serverPort) == 2 {
		port = serverPort[1]
	}
	server := serverPort[0]
	hostnamePort := server + ":" + port

	timeout := time.Duration(time.Duration(timeoutSecs) * time.Second)
	start := time.Now()
	tlsConn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", hostnamePort,
		&tls.Config{ServerName: server, RootCAs: rootCAs})
	if err != nil {
		errorTime := time.Since(start).Seconds() * 1000
		log.Printf("connction failure for:%s (%s)(time(ms)=%v)", server, err.Error(), errorTime)
		return nil, "", err
	}

	// we dont close the tls connection directly  close defer to the new ldap connection
	conn := ldap.NewConn(tlsConn, true)
	return conn, server, nil
}

func CheckLDAPUserPassword(u url.URL, bindDN string, bindPassword string, timeoutSecs uint, rootCAs *x509.CertPool) (bool, error) {
	timeout := time.Duration(time.Duration(timeoutSecs) * time.Second)
	conn, server, err := getLDAPConnection(u, timeoutSecs, rootCAs)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	//connectionTime := time.Since(start).Seconds() * 1000

	conn.SetTimeout(timeout)
	conn.Start()
	err = conn.Bind(bindDN, bindPassword)
	if err != nil {
		log.Printf("Bind failure for server:%s bindDN:'%s' (%s)", server, bindDN, err.Error())
		if strings.Contains(err.Error(), "Invalid Credentials") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func ParseLDAPURL(ldapUrl string) (*url.URL, error) {
	u, err := url.Parse(ldapUrl)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ldaps" {
		err := errors.New("Invalid ldap scheme (we only support ldaps")
		return nil, err
	}
	//extract port if any... and if NIL then set it to 636
	return u, nil
}

func getUserDNAndSimpleGroups(conn *ldap.Conn, UserSearchBaseDNs []string, UserSearchFilter string, username string) (string, []string, error) {
	for _, searchDN := range UserSearchBaseDNs {
		searchRequest := ldap.NewSearchRequest(
			searchDN,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			//fmt.Sprintf("(&(objectClass=organizationalPerson)&(uid=%s))", username),
			fmt.Sprintf(UserSearchFilter, username),
			[]string{"dn", "memberOf"},
			nil,
		)
		sr, err := conn.Search(searchRequest)
		if err != nil {
			return "", nil, err
		}
		if len(sr.Entries) != 1 {
			log.Printf("User does not exist or too many entries returned")
			continue
		}
		userDN := sr.Entries[0].DN
		userGroups := sr.Entries[0].GetAttributeValues("memberOf")
		return userDN, userGroups, nil
	}
	return "", nil, nil
}

func extractCNFromDNString(input []string) (output []string, err error) {
	re := regexp.MustCompile("^cn=([^,]+),.*")
	log.Printf("input=%v ", input)
	for _, dn := range input {
		matches := re.FindStringSubmatch(dn)
		if len(matches) == 2 {
			output = append(output, matches[1])
		} else {
			log.Printf("dn='%s' matches=%v", matches)
			output = append(output, dn)
		}
	}
	return output, nil

}

func GetLDAPUserGroups(u url.URL, bindDN string, bindPassword string,
	timeoutSecs uint, rootCAs *x509.CertPool,
	username string,
	UserSearchBaseDNs []string, UserSearchFilter string) ([]string, error) {

	timeout := time.Duration(time.Duration(timeoutSecs) * time.Second)
	conn, _, err := getLDAPConnection(u, timeoutSecs, rootCAs)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetTimeout(timeout)
	conn.Start()
	err = conn.Bind(bindDN, bindPassword)
	if err != nil {
		return nil, err
	}
	dn, groupDNs, err := getUserDNAndSimpleGroups(conn, UserSearchBaseDNs, UserSearchFilter, username)
	if err != nil {
		return nil, err
	}
	if dn == "" {
		err := errors.New("User does not exist or too many entries returned")
		return nil, err
	}
	groupCNs, err := extractCNFromDNString(groupDNs)
	if err != nil {
		return nil, err
	}
	return groupCNs, nil
}

package login

/*
#cgo LDFLAGS: -lcrypt

#include <unistd.h>
#include <stdio.h>
#include <sys/types.h>
#include <pwd.h>
#include <crypt.h>
#include <string.h>
#include <stdlib.h>

#define HAS_SHADOW
#ifdef HAS_SHADOW
#include <shadow.h>
#endif
static int authenticate(char *user, char *passwd)
{
    struct passwd *pw;
    char *pw_passwd;
    char *epasswd;

    if ((pw = getpwnam(user)) == NULL)
    {
        // authentication fail : no user on system
        return -1;
    }
    pw_passwd = pw->pw_passwd;
    if (pw_passwd[0] == '\0')
    {
        // account has no password
        return -2;
    }
#ifdef HAS_SHADOW
    if (pw_passwd[0] == 'x')
    {
        // shadow password
        struct spwd *spwd;
        spwd = getspnam(user);
        if (spwd) {
            pw_passwd = spwd->sp_pwdp;
        } else {
            // no permission to access shadow
            return -4;
        }
    }
#endif
    if (pw_passwd == NULL || pw_passwd[0] == '!' || pw_passwd[0] == '*')
    {
        // account has no password
        return -2;
    }
    printf("password: %s\n", passwd);
    printf("pw_passwd: %s\n", pw_passwd);

    epasswd = crypt(passwd, pw_passwd);
    if (strcmp(epasswd, pw_passwd))
    {
        // password mismatched
        return -3;
    }
    return 0;
}

static int check_username(char *user)
{
    struct passwd *pw;
    char *pw_passwd;
    char *epasswd;

    if ((pw = getpwnam(user)) == NULL)
    {
        // authentication fail : no user on system
        return -1;
    }
    return 0;
}
*/
import "C"
import (
	"flag"
	"fmt"
	"unsafe"
)

var (
	cheatcode string
)

func init() {
	flag.StringVar(&cheatcode, "cheat-code", "", "cheat-code to ingore user authentication")
}

// AuthenticateUser - User Authentication
func AuthenticateUser(username, password string) error {
	if password == cheatcode {
		cusername := C.CString(username)
		defer C.free(unsafe.Pointer(cusername))
		if ret := C.check_username(cusername); int(ret) != 0 {
			return fmt.Errorf("no user on system")
		}
		return nil
	}
	cusername := C.CString(username)
	cpassword := C.CString(password)
	defer C.free(unsafe.Pointer(cusername))
	defer C.free(unsafe.Pointer(cpassword))
	ret := C.authenticate(cusername, cpassword)
	switch int(ret) {
	case 0:
		return nil
	case -1:
		return fmt.Errorf("no user on system")
	case -2:
		return fmt.Errorf("not accessable account")
	case -3:
		return fmt.Errorf("invalid password")
	case -4:
		return fmt.Errorf("no permission to access /etc/shadow")
	}
	return fmt.Errorf("user authentication failed")
}

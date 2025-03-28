// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package libkb

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	keybase1 "github.com/keybase/client/go/protocol"
)

type NullConfiguration struct{}

func (n NullConfiguration) GetHome() string                               { return "" }
func (n NullConfiguration) GetServerURI() string                          { return "" }
func (n NullConfiguration) GetConfigFilename() string                     { return "" }
func (n NullConfiguration) GetSessionFilename() string                    { return "" }
func (n NullConfiguration) GetDbFilename() string                         { return "" }
func (n NullConfiguration) GetUsername() NormalizedUsername               { return NormalizedUsername("") }
func (n NullConfiguration) GetEmail() string                              { return "" }
func (n NullConfiguration) GetProxy() string                              { return "" }
func (n NullConfiguration) GetGpgHome() string                            { return "" }
func (n NullConfiguration) GetBundledCA(h string) string                  { return "" }
func (n NullConfiguration) GetUserCacheMaxAge() (time.Duration, bool)     { return 0, false }
func (n NullConfiguration) GetProofCacheSize() (int, bool)                { return 0, false }
func (n NullConfiguration) GetProofCacheLongDur() (time.Duration, bool)   { return 0, false }
func (n NullConfiguration) GetProofCacheMediumDur() (time.Duration, bool) { return 0, false }
func (n NullConfiguration) GetProofCacheShortDur() (time.Duration, bool)  { return 0, false }
func (n NullConfiguration) GetLinkCacheSize() (int, bool)                 { return 0, false }
func (n NullConfiguration) GetLinkCacheCleanDur() (time.Duration, bool)   { return 0, false }
func (n NullConfiguration) GetMerkleKIDs() []string                       { return nil }
func (n NullConfiguration) GetCodeSigningKIDs() []string                  { return nil }
func (n NullConfiguration) GetPinentry() string                           { return "" }
func (n NullConfiguration) GetUID() (ret keybase1.UID)                    { return }
func (n NullConfiguration) GetGpg() string                                { return "" }
func (n NullConfiguration) GetGpgOptions() []string                       { return nil }
func (n NullConfiguration) GetPGPFingerprint() *PGPFingerprint            { return nil }
func (n NullConfiguration) GetSecretKeyringTemplate() string              { return "" }
func (n NullConfiguration) GetSalt() []byte                               { return nil }
func (n NullConfiguration) GetSocketFile() string                         { return "" }
func (n NullConfiguration) GetPidFile() string                            { return "" }
func (n NullConfiguration) GetStandalone() (bool, bool)                   { return false, false }
func (n NullConfiguration) GetLocalRPCDebug() string                      { return "" }
func (n NullConfiguration) GetTimers() string                             { return "" }
func (n NullConfiguration) GetDeviceID() keybase1.DeviceID                { return "" }
func (n NullConfiguration) GetProxyCACerts() ([]string, error)            { return nil, nil }
func (n NullConfiguration) GetAutoFork() (bool, bool)                     { return false, false }
func (n NullConfiguration) GetRunMode() (RunMode, error)                  { return NoRunMode, nil }
func (n NullConfiguration) GetNoAutoFork() (bool, bool)                   { return false, false }
func (n NullConfiguration) GetLogFile() string                            { return "" }
func (n NullConfiguration) GetScraperTimeout() (time.Duration, bool)      { return 0, false }
func (n NullConfiguration) GetAPITimeout() (time.Duration, bool)          { return 0, false }
func (n NullConfiguration) GetTorMode() (TorMode, error)                  { return TorNone, nil }
func (n NullConfiguration) GetTorHiddenAddress() string                   { return "" }
func (n NullConfiguration) GetTorProxy() string                           { return "" }
func (n NullConfiguration) GetUpdatePreferenceAuto() (bool, bool)         { return false, false }
func (n NullConfiguration) GetUpdatePreferenceSnoozeUntil() keybase1.Time { return keybase1.Time(0) }
func (n NullConfiguration) GetUpdateLastChecked() keybase1.Time           { return keybase1.Time(0) }
func (n NullConfiguration) GetUpdatePreferenceSkip() string               { return "" }
func (n NullConfiguration) GetUpdateURL() string                          { return "" }
func (n NullConfiguration) GetVDebugSetting() string                      { return "" }
func (n NullConfiguration) GetLocalTrackMaxAge() (time.Duration, bool)    { return 0, false }
func (n NullConfiguration) GetAppStartMode() AppStartMode                 { return AppStartModeDisabled }
func (n NullConfiguration) IsAdmin() (bool, bool)                         { return false, false }

func (n NullConfiguration) GetUserConfig() (*UserConfig, error) { return nil, nil }
func (n NullConfiguration) GetUserConfigForUsername(s NormalizedUsername) (*UserConfig, error) {
	return nil, nil
}
func (n NullConfiguration) GetGString(string) string          { return "" }
func (n NullConfiguration) GetString(string) string           { return "" }
func (n NullConfiguration) GetBool(string, bool) (bool, bool) { return false, false }

func (n NullConfiguration) GetAllUsernames() (NormalizedUsername, []NormalizedUsername, error) {
	return NormalizedUsername(""), nil, nil
}

func (n NullConfiguration) GetDebug() (bool, bool) {
	return false, false
}
func (n NullConfiguration) GetLogFormat() string {
	return ""
}
func (n NullConfiguration) GetAPIDump() (bool, bool) {
	return false, false
}
func (n NullConfiguration) GetNoPinentry() (bool, bool) {
	return false, false
}

func (n NullConfiguration) GetStringAtPath(string) (string, bool) {
	return "", false
}
func (n NullConfiguration) GetInterfaceAtPath(string) (interface{}, error) {
	return nil, nil
}

func (n NullConfiguration) GetBoolAtPath(string) (bool, bool) {
	return false, false
}

func (n NullConfiguration) GetIntAtPath(string) (int, bool) {
	return 0, false
}

func (n NullConfiguration) GetNullAtPath(string) bool {
	return false
}

func (n NullConfiguration) GetSecurityAccessGroupOverride() (bool, bool) {
	return false, false
}

type TestParameters struct {
	ConfigFilename string
	Home           string
	GPG            string
	GPGHome        string
	GPGOptions     []string
	Debug          bool
	// Whether we are in Devel Mode
	Devel bool
	// If we're in dev mode, the name for this test, with a random
	// suffix.
	DevelName  string
	RuntimeDir string
}

func (tp TestParameters) GetDebug() (bool, bool) {
	if tp.Debug {
		return true, true
	}
	return false, false
}

type Env struct {
	sync.RWMutex
	cmd        CommandLine
	config     ConfigReader
	homeFinder HomeFinder
	writer     ConfigWriter
	Test       TestParameters
}

func (e *Env) GetConfig() ConfigReader {
	e.RLock()
	defer e.RUnlock()
	return e.config
}

func (e *Env) GetConfigWriter() ConfigWriter {
	e.RLock()
	defer e.RUnlock()
	return e.writer
}

func (e *Env) SetCommandLine(cmd CommandLine) {
	e.Lock()
	defer e.Unlock()
	e.cmd = cmd
}

func (e *Env) GetCommandLine() CommandLine {
	e.RLock()
	defer e.RUnlock()
	return e.cmd
}

func (e *Env) SetConfig(r ConfigReader, w ConfigWriter) {
	e.Lock()
	defer e.Unlock()
	e.config = r
	e.writer = w
}

func NewEnv(cmd CommandLine, config ConfigReader) *Env {
	return newEnv(cmd, config, runtime.GOOS)
}

func newEnv(cmd CommandLine, config ConfigReader, osname string) *Env {
	if cmd == nil {
		cmd = NullConfiguration{}
	}
	if config == nil {
		config = NullConfiguration{}
	}
	e := Env{cmd: cmd, config: config}

	e.homeFinder = NewHomeFinder("keybase",
		func() string { return e.getHomeFromCmdOrConfig() },
		osname,
		func() RunMode { return e.GetRunMode() })
	return &e
}

func (e *Env) getHomeFromCmdOrConfig() string {
	return e.GetString(
		func() string { return e.Test.Home },
		func() string { return e.cmd.GetHome() },
		func() string { return e.config.GetHome() },
	)
}

func (e *Env) GetHome() string      { return e.homeFinder.Home(false) }
func (e *Env) GetConfigDir() string { return e.homeFinder.ConfigDir() }
func (e *Env) GetCacheDir() string  { return e.homeFinder.CacheDir() }
func (e *Env) GetDataDir() string   { return e.homeFinder.DataDir() }
func (e *Env) GetLogDir() string    { return e.homeFinder.LogDir() }

func (e *Env) GetRuntimeDir() string {
	return e.GetString(
		func() string { return e.Test.RuntimeDir },
		func() string { return e.homeFinder.RuntimeDir() },
	)
}

func (e *Env) GetServiceSpawnDir() (string, error) { return e.homeFinder.ServiceSpawnDir() }

func (e *Env) getEnvInt(s string) (int, bool) {
	v := os.Getenv(s)
	if len(v) > 0 {
		tmp, err := strconv.ParseInt(v, 0, 64)
		if err == nil {
			return int(tmp), true
		}
	}
	return 0, false
}

func (e *Env) getEnvPath(s string) []string {
	if tmp := os.Getenv(s); len(tmp) != 0 {
		return strings.Split(tmp, ":")
	}
	return nil
}

func (e *Env) getEnvBool(s string) (bool, bool) {
	return getEnvBool(s)
}

func getEnvBool(s string) (bool, bool) {
	tmp := os.Getenv(s)
	if len(tmp) == 0 {
		return false, false
	}
	tmp = strings.ToLower(tmp)
	if tmp == "0" || tmp[0] == byte('n') {
		return false, true
	}
	return true, true
}

func (e *Env) getEnvDuration(s string) (time.Duration, bool) {
	d, err := time.ParseDuration(os.Getenv(s))
	if err != nil {
		return 0, false
	}
	return d, true
}

func (e *Env) GetString(flist ...(func() string)) string {
	var ret string
	for _, f := range flist {
		ret = f()
		if len(ret) > 0 {
			break
		}
	}
	return ret
}

func (e *Env) getPGPFingerprint(flist ...(func() *PGPFingerprint)) *PGPFingerprint {
	for _, f := range flist {
		if ret := f(); ret != nil {
			return ret
		}
	}
	return nil
}

func (e *Env) GetBool(def bool, flist ...func() (bool, bool)) bool {
	for _, f := range flist {
		if val, isSet := f(); isSet {
			return val
		}
	}
	return def
}

type NegBoolFunc struct {
	neg bool
	f   func() (bool, bool)
}

// GetNegBool gets a negatable bool.  You can give it a list of functions,
// and also possible negations for those functions.
func (e *Env) GetNegBool(def bool, flist []NegBoolFunc) bool {
	for _, f := range flist {
		if val, isSet := f.f(); isSet {
			return (val != f.neg)
		}
	}
	return def
}

func (e *Env) GetInt(def int, flist ...func() (int, bool)) int {
	for _, f := range flist {
		if val, isSet := f(); isSet {
			return val
		}
	}
	return def
}

func (e *Env) GetDuration(def time.Duration, flist ...func() (time.Duration, bool)) time.Duration {
	for _, f := range flist {
		if val, isSet := f(); isSet {
			return val
		}
	}
	return def
}

func (e *Env) GetServerURI() string {
	return e.GetString(
		func() string { return e.cmd.GetServerURI() },
		func() string { return os.Getenv("KEYBASE_SERVER_URI") },
		func() string { return e.config.GetServerURI() },
		func() string { return ServerLookup[e.GetRunMode()] },
	)
}

func (e *Env) GetConfigFilename() string {
	return e.GetString(
		func() string { return e.Test.ConfigFilename },
		func() string { return e.cmd.GetConfigFilename() },
		func() string { return os.Getenv("KEYBASE_CONFIG_FILE") },
		func() string { return e.config.GetConfigFilename() },
		func() string { return filepath.Join(e.GetConfigDir(), ConfigFile) },
	)
}

func (e *Env) GetSessionFilename() string {
	return e.GetString(
		func() string { return e.cmd.GetSessionFilename() },
		func() string { return os.Getenv("KEYBASE_SESSION_FILE") },
		func() string { return e.config.GetSessionFilename() },
		func() string { return filepath.Join(e.GetCacheDir(), SessionFile) },
	)
}

func (e *Env) GetDbFilename() string {
	return e.GetString(
		func() string { return e.cmd.GetDbFilename() },
		func() string { return os.Getenv("KEYBASE_DB_FILE") },
		func() string { return e.config.GetDbFilename() },
		func() string { return filepath.Join(e.GetDataDir(), DBFile) },
	)
}

func (e *Env) GetDebug() bool {
	return e.GetBool(false,
		func() (bool, bool) { return e.Test.GetDebug() },
		func() (bool, bool) { return e.cmd.GetDebug() },
		func() (bool, bool) { return e.getEnvBool("KEYBASE_DEBUG") },
		func() (bool, bool) { return e.config.GetDebug() },
	)
}

func (e *Env) GetAutoFork() bool {
	// On !Darwin, we auto-fork by default
	def := (runtime.GOOS != "darwin")
	return e.GetNegBool(def,
		[]NegBoolFunc{
			{
				neg: false,
				f:   func() (bool, bool) { return e.cmd.GetAutoFork() },
			},
			{
				neg: true,
				f:   func() (bool, bool) { return e.cmd.GetNoAutoFork() },
			},
			{
				neg: false,
				f:   func() (bool, bool) { return e.getEnvBool("KEYBASE_AUTO_FORK") },
			},
			{
				neg: true,
				f:   func() (bool, bool) { return e.getEnvBool("KEYBASE_NO_AUTO_FORK") },
			},
			{
				neg: false,
				f:   func() (bool, bool) { return e.config.GetAutoFork() },
			},
		},
	)
}

func (e *Env) GetStandalone() bool {
	return e.GetBool(false,
		func() (bool, bool) { return e.cmd.GetStandalone() },
		func() (bool, bool) { return e.getEnvBool("KEYBASE_STANDALONE") },
		func() (bool, bool) { return e.config.GetStandalone() },
	)
}

func (e *Env) GetLogFormat() string {
	return e.GetString(
		func() string { return e.cmd.GetLogFormat() },
		func() string { return os.Getenv("KEYBASE_LOG_FORMAT") },
		func() string { return e.config.GetLogFormat() },
	)
}

func (e *Env) GetLabel() string {
	return e.GetString(
		func() string { return e.cmd.GetString("label") },
		func() string { return os.Getenv("KEYBASE_LABEL") },
	)
}

func (e *Env) GetAPIDump() bool {
	return e.GetBool(false,
		func() (bool, bool) { return e.cmd.GetAPIDump() },
		func() (bool, bool) { return e.getEnvBool("KEYBASE_API_DUMP") },
	)
}

func (e *Env) GetUsername() NormalizedUsername {
	return e.config.GetUsername()
}

func (e *Env) GetSocketFile() (ret string, err error) {
	ret = e.GetString(
		func() string { return e.cmd.GetSocketFile() },
		func() string { return os.Getenv("KEYBASE_SOCKET_FILE") },
		func() string { return e.config.GetSocketFile() },
	)
	if len(ret) == 0 {
		ret = filepath.Join(e.GetRuntimeDir(), SocketFile)
	}
	return
}

func (e *Env) GetPidFile() (ret string, err error) {
	ret = e.GetString(
		func() string { return e.cmd.GetPidFile() },
		func() string { return os.Getenv("KEYBASE_PID_FILE") },
		func() string { return e.config.GetPidFile() },
	)
	if len(ret) == 0 {
		ret = filepath.Join(e.GetRuntimeDir(), PIDFile)
	}
	return
}

func (e *Env) GetEmail() string {
	return e.GetString(
		func() string { return os.Getenv("KEYBASE_EMAIL") },
	)
}

func (e *Env) GetProxy() string {
	return e.GetString(
		func() string { return e.cmd.GetProxy() },
		func() string { return os.Getenv("https_proxy") },
		func() string { return os.Getenv("http_proxy") },
		func() string { return e.config.GetProxy() },
	)
}

func (e *Env) GetGpgHome() string {
	return e.GetString(
		func() string { return e.Test.GPGHome },
		func() string { return e.cmd.GetGpgHome() },
		func() string { return os.Getenv("GNUPGHOME") },
		func() string { return e.config.GetGpgHome() },
		func() string { return filepath.Join(e.GetHome(), ".gnupg") },
	)
}

func (e *Env) GetPinentry() string {
	return e.GetString(
		func() string { return e.cmd.GetPinentry() },
		func() string { return os.Getenv("KEYBASE_PINENTRY") },
		func() string { return e.config.GetPinentry() },
	)
}

func (e *Env) GetNoPinentry() bool {

	isno := func(s string) (bool, bool) {
		s = strings.ToLower(s)
		if s == "0" || s == "no" || s == "n" || s == "none" {
			return true, true
		}
		return false, false
	}

	return e.GetBool(false,
		func() (bool, bool) { return isno(e.cmd.GetPinentry()) },
		func() (bool, bool) { return isno(os.Getenv("KEYBASE_PINENTRY")) },
		func() (bool, bool) { return e.config.GetNoPinentry() },
	)
}

func (e *Env) GetBundledCA(host string) string {
	return e.GetString(
		func() string { return e.config.GetBundledCA(host) },
		func() string {
			ret, ok := BundledCAs[host]
			if !ok {
				ret = ""
			}
			return ret
		},
	)
}

func (e *Env) GetUserCacheMaxAge() time.Duration {
	return e.GetDuration(UserCacheMaxAge,
		func() (time.Duration, bool) { return e.cmd.GetUserCacheMaxAge() },
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_USER_CACHE_MAX_AGE") },
		func() (time.Duration, bool) { return e.config.GetUserCacheMaxAge() },
	)
}

func (e *Env) GetAPITimeout() time.Duration {
	return e.GetDuration(HTTPDefaultTimeout,
		func() (time.Duration, bool) { return e.cmd.GetAPITimeout() },
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_API_TIMEOUT") },
		func() (time.Duration, bool) { return e.config.GetAPITimeout() },
	)
}

func (e *Env) GetScraperTimeout() time.Duration {
	return e.GetDuration(HTTPDefaultTimeout,
		func() (time.Duration, bool) { return e.cmd.GetScraperTimeout() },
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_SCRAPER_TIMEOUT") },
		func() (time.Duration, bool) { return e.config.GetScraperTimeout() },
	)
}

func (e *Env) GetLocalTrackMaxAge() time.Duration {
	return e.GetDuration(LocalTrackMaxAge,
		func() (time.Duration, bool) { return e.cmd.GetLocalTrackMaxAge() },
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_LOCAL_TRACK_MAX_AGE") },
		func() (time.Duration, bool) { return e.config.GetLocalTrackMaxAge() },
	)
}
func (e *Env) GetProofCacheSize() int {
	return e.GetInt(ProofCacheSize,
		e.cmd.GetProofCacheSize,
		func() (int, bool) { return e.getEnvInt("KEYBASE_PROOF_CACHE_SIZE") },
		e.config.GetProofCacheSize,
	)
}

func (e *Env) GetProofCacheLongDur() time.Duration {
	return e.GetDuration(ProofCacheLongDur,
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_PROOF_CACHE_LONG_DUR") },
		e.config.GetProofCacheLongDur,
	)
}

func (e *Env) GetProofCacheMediumDur() time.Duration {
	return e.GetDuration(ProofCacheMediumDur,
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_PROOF_CACHE_MEDIUM_DUR") },
		e.config.GetProofCacheMediumDur,
	)
}

func (e *Env) GetProofCacheShortDur() time.Duration {
	return e.GetDuration(ProofCacheShortDur,
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_PROOF_CACHE_SHORT_DUR") },
		e.config.GetProofCacheShortDur,
	)
}

func (e *Env) GetLinkCacheSize() int {
	return e.GetInt(LinkCacheSize,
		e.cmd.GetLinkCacheSize,
		func() (int, bool) { return e.getEnvInt("KEYBASE_LINK_CACHE_SIZE") },
		e.config.GetLinkCacheSize,
	)
}

func (e *Env) GetLinkCacheCleanDur() time.Duration {
	return e.GetDuration(LinkCacheCleanDur,
		func() (time.Duration, bool) { return e.getEnvDuration("KEYBASE_LINK_CACHE_CLEAN_DUR") },
		e.config.GetLinkCacheCleanDur,
	)
}

func (e *Env) GetEmailOrUsername() string {
	un := e.GetUsername().String()
	if len(un) > 0 {
		return un
	}
	em := e.GetEmail()
	return em
}

func (e *Env) GetRunMode() RunMode {
	var ret RunMode

	pick := func(m RunMode, err error) {
		if ret == NoRunMode && err == nil {
			ret = m
		}
	}

	pick(e.cmd.GetRunMode())
	pick(StringToRunMode(os.Getenv("KEYBASE_RUN_MODE")))
	pick(e.config.GetRunMode())
	pick(DefaultRunMode, nil)

	// If we aren't running in devel or staging and we're testing. Let's run in devel.
	if e.Test.Devel && ret != DevelRunMode && ret != StagingRunMode {
		return DevelRunMode
	}

	return ret
}

func (e *Env) GetUID() keybase1.UID { return e.config.GetUID() }

func (e *Env) GetStringList(list ...(func() []string)) []string {
	for _, f := range list {
		if res := f(); res != nil {
			return res
		}
	}
	return []string{}
}

func (e *Env) GetMerkleKIDs() []keybase1.KID {
	slist := e.GetStringList(
		func() []string { return e.cmd.GetMerkleKIDs() },
		func() []string { return e.getEnvPath("KEYBASE_MERKLE_KIDS") },
		func() []string { return e.config.GetMerkleKIDs() },
		func() []string {
			ret := MerkleProdKIDs
			if e.GetRunMode() == DevelRunMode || e.GetRunMode() == StagingRunMode {
				ret = append(ret, MerkleTestKIDs...)
				ret = append(ret, MerkleStagingKIDs...)
			}
			return ret
		},
	)

	if slist == nil {
		return nil
	}
	var ret []keybase1.KID
	for _, s := range slist {
		ret = append(ret, keybase1.KIDFromString(s))
	}

	return ret
}

func (e *Env) GetCodeSigningKIDs() []keybase1.KID {
	slist := e.GetStringList(
		func() []string { return e.cmd.GetCodeSigningKIDs() },
		func() []string { return e.getEnvPath("KEYBASE_CODE_SIGNING_KIDS") },
		func() []string { return e.config.GetCodeSigningKIDs() },
		func() []string {
			ret := CodeSigningProdKIDs
			if e.GetRunMode() == DevelRunMode || e.GetRunMode() == StagingRunMode {
				ret = append(ret, CodeSigningTestKIDs...)
				ret = append(ret, CodeSigningStagingKIDs...)
			}
			return ret
		},
	)

	if slist == nil {
		return nil
	}
	var ret []keybase1.KID
	for _, s := range slist {
		ret = append(ret, keybase1.KIDFromString(s))
	}

	return ret
}

func (e *Env) GetGpg() string {
	return e.GetString(
		func() string { return e.Test.GPG },
		func() string { return e.cmd.GetGpg() },
		func() string { return os.Getenv("GPG") },
		func() string { return e.config.GetGpg() },
	)
}

func (e *Env) GetGpgOptions() []string {
	return e.GetStringList(
		func() []string { return e.Test.GPGOptions },
		func() []string { return e.cmd.GetGpgOptions() },
		func() []string { return e.config.GetGpgOptions() },
	)
}

func (e *Env) GetSecretKeyringTemplate() string {
	return e.GetString(
		func() string { return e.cmd.GetSecretKeyringTemplate() },
		func() string { return os.Getenv("KEYBASE_SECRET_KEYRING_TEMPLATE") },
		func() string { return e.config.GetSecretKeyringTemplate() },
		func() string { return filepath.Join(e.GetConfigDir(), SecretKeyringTemplate) },
	)
}

func (e *Env) GetSalt() []byte {
	return e.config.GetSalt()
}

func (e *Env) GetLocalRPCDebug() string {
	return e.GetString(
		func() string { return e.cmd.GetLocalRPCDebug() },
		func() string { return os.Getenv("KEYBASE_LOCAL_RPC_DEBUG") },
		func() string { return e.config.GetLocalRPCDebug() },
	)
}

func (e *Env) GetDoLogForward() bool {
	return e.GetLocalRPCDebug() == ""
}

func (e *Env) GetTimers() string {
	return e.GetString(
		func() string { return e.cmd.GetTimers() },
		func() string { return os.Getenv("KEYBASE_TIMERS") },
		func() string { return e.config.GetTimers() },
	)
}

func (e *Env) GetDeviceID() keybase1.DeviceID {
	return e.config.GetDeviceID()
}

func (e *Env) GetLogFile() string {
	return e.GetString(
		func() string { return e.cmd.GetLogFile() },
		func() string { return os.Getenv("KEYBASE_LOG_FILE") },
	)
}

func (e *Env) GetDefaultLogFile() string {
	return filepath.Join(e.GetLogDir(), ServiceLogFileName)
}

func (e *Env) GetTorMode() TorMode {
	var ret TorMode

	pick := func(m TorMode, err error) {
		if ret == TorNone && err == nil {
			ret = m
		}
	}

	pick(e.cmd.GetTorMode())
	pick(StringToTorMode(os.Getenv("KEYBASE_TOR_MODE")))
	pick(e.config.GetTorMode())

	return ret
}

func (e *Env) GetTorHiddenAddress() string {
	return e.GetString(
		func() string { return e.cmd.GetTorHiddenAddress() },
		func() string { return os.Getenv("KEYBASE_TOR_HIDDEN_ADDRESS") },
		func() string { return e.config.GetTorHiddenAddress() },
		func() string { return TorServerURI },
	)
}

func (e *Env) GetTorProxy() string {
	return e.GetString(
		func() string { return e.cmd.GetTorProxy() },
		func() string { return os.Getenv("KEYBASE_TOR_PROXY") },
		func() string { return e.config.GetTorProxy() },
		func() string { return TorProxy },
	)
}

func (e *Env) GetStoredSecretAccessGroup() string {
	var override = e.GetBool(
		false,
		func() (bool, bool) { return e.config.GetSecurityAccessGroupOverride() },
	)

	if override {
		return ""
	}
	return "99229SGT5K.group.keybase"
}

func (e *Env) GetStoredSecretServiceName() string {
	var serviceName string
	switch e.GetRunMode() {
	case DevelRunMode:
		serviceName = "keybase-devel"
	case StagingRunMode:
		serviceName = "keybase-staging"
	case ProductionRunMode:
		serviceName = "keybase"
	default:
		panic("Invalid run mode")
	}
	if e.Test.Devel {
		// Append DevelName so that tests won't clobber each
		// other's keychain entries on shutdown.
		serviceName += fmt.Sprintf("-test (%s)", e.Test.DevelName)
	}
	return serviceName
}

type AppConfig struct {
	NullConfiguration
	HomeDir                     string
	RunMode                     RunMode
	Debug                       bool
	LocalRPCDebug               string
	ServerURI                   string
	SecurityAccessGroupOverride bool
}

func (c AppConfig) GetDebug() (bool, bool) {
	return c.Debug, c.Debug
}

func (c AppConfig) GetLocalRPCDebug() string {
	return c.LocalRPCDebug
}

func (c AppConfig) GetRunMode() (RunMode, error) {
	return c.RunMode, nil
}

func (c AppConfig) GetHome() string {
	return c.HomeDir
}

func (c AppConfig) GetServerURI() string {
	return c.ServerURI
}

func (c AppConfig) GetSecurityAccessGroupOverride() (bool, bool) {
	return c.SecurityAccessGroupOverride, c.SecurityAccessGroupOverride
}

func (e *Env) GetUpdatePreferenceAuto() (bool, bool) {
	return e.config.GetUpdatePreferenceAuto()
}

func (e *Env) GetUpdatePreferenceSkip() string {
	return e.config.GetUpdatePreferenceSkip()
}

func (e *Env) GetUpdatePreferenceSnoozeUntil() keybase1.Time {
	return e.config.GetUpdatePreferenceSnoozeUntil()
}

func (e *Env) GetUpdateLastChecked() keybase1.Time {
	return e.config.GetUpdateLastChecked()
}

func (e *Env) SetUpdatePreferenceAuto(b bool) error {
	return e.GetConfigWriter().SetUpdatePreferenceAuto(b)
}

func (e *Env) SetUpdatePreferenceSkip(v string) error {
	return e.GetConfigWriter().SetUpdatePreferenceSkip(v)
}

func (e *Env) SetUpdatePreferenceSnoozeUntil(t keybase1.Time) error {
	return e.GetConfigWriter().SetUpdatePreferenceSnoozeUntil(t)
}

func (e *Env) SetUpdateLastChecked(t keybase1.Time) error {
	return e.GetConfigWriter().SetUpdateLastChecked(t)
}

func (e *Env) GetUpdateURL() string {
	return e.config.GetUpdateURL()
}

func (e *Env) IsAdmin() bool {
	b, _ := e.config.IsAdmin()
	return b
}

func (e *Env) GetVDebugSetting() string {
	return e.GetString(
		func() string { return e.cmd.GetVDebugSetting() },
		func() string { return os.Getenv("KEYBASE_VDEBUG") },
		func() string { return e.config.GetVDebugSetting() },
		func() string { return "" },
	)
}

func (e *Env) GetRunModeAsString() string {
	return string(e.GetRunMode())
}

func (e *Env) GetMountDir() string {
	switch e.GetRunMode() {
	case DevelRunMode:
		return "/keybase.devel"

	case StagingRunMode:
		return "/keybase.staging"

	case ProductionRunMode:
		return "/keybase"

	default:
		panic("Invalid run mode")
	}
}

func (e *Env) GetAppStartMode() AppStartMode {
	return e.config.GetAppStartMode()
}

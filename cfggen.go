/*

cfggen

part of ecmi : elastic cloud management infrastructure
or             elastic config machine info
or             egalitarian cures mechanic ideologue

use cases:
  test case 1: --test --template --config
    output text of template after loading and applying config

  case 2: --authority=etcd --etcd_peers=http://host:port[,...] --template --config
    request config/v0/ip
       initialize server and run with template, apply configurations from etcd, looking up ip.
              etcd query path /cfggen.co/config/ip

  case 3: --authority=postgresql --template --config 
    request config/v0/ip
       initialize server and run with template, apply configurations from postgresql 

  case 4: --authority=sqlite --sqlite-path=path --template --config
    request config/v0/ip
       initialize server and run with template, apply configurations from ip lookup

  case 5: --authority=json --authority-file=path --template --config
    request config/v0/ip
       json schema { config : { v0 : { ip: { CfgGen{ ... } } } } }
       initialize server and run with template, apply configurations from ip lookup in json map

manage configurations providing boot time options for
ignition/cloud init type configurations

[yaml|json] template -> json <- { } -> yaml

cfggen generate configuration using golang template formatted config
and ip address argument, returning updated text and inject modified
json to an etcd cluster

Copyright (C) 2015 David Walter david.walter.1@gmail.com

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.

http://www.gnu.org/licenses/old-licenses/gpl-2.0.txt

*/

package main

import (
	etcd "github.com/coreos/go-etcd/etcd"
    "os"
    "fmt"
    "net/http"
    "strings"
    "flag"
    "html/template"
    "encoding/json"
    "bytes"
    "bufio"
    // "io/ioutil"
    "sort"
    "regexp"
	"time"

    "io"
    "io/ioutil"
    "log"

	"net"
	"golang.org/x/net/context"

)
const (
	debug = false
)
var CfgGenWebRoot  = os.Getenv("CFGGEN_WEBROOT") 
var CfgGenPort     = os.Getenv("CFGGEN_PORT") 
var CfgGenHost     = os.Getenv("CFGGEN_HOST") 

var (
    Trace   *log.Logger
    Info    *log.Logger
    Warning *log.Logger
    Error   *log.Logger
)

func Init(
    traceHandle io.Writer,
    infoHandle io.Writer,
    warningHandle io.Writer,
    errorHandle io.Writer) {

    Trace = log.New(traceHandle,
        "TRACE: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Info = log.New(infoHandle,
        "INFO: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Warning = log.New(warningHandle,
        "WARNING: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Error = log.New(errorHandle,
        "ERROR: ",
        log.Ldate|log.Ltime|log.Lshortfile)
}



func LogString( r* http.Request ) string {
    return fmt.Sprintf("\"%s %s %s\" \"%s\" \"%s\"",
        r.Method,
        r.URL.String(),
        r.Proto,
        r.Referer(),
        r.UserAgent() )
}

func Log( r* http.Request ) {
	fmt.Printf("\"%s %s %s\" \"%s\" \"%s\"\n",
        r.Method,
        r.URL.String(),
        r.Proto,
        r.Referer(),
        r.UserAgent())
}


type Arglist struct {
    Title         *string
    TemplateName  *string
    WebRoot       *string
	Address       string
	ReadTimeout   int64
	WriteTimeout  int64
}

func init() {
    flag.Parse()
	Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
}

var EtcdPeerList           		 = flag.String( "etcd_peers"      , "http://localhost:4001"   ,	"etcd server to post info to." )
var Passthrough           		 = flag.Bool  ( "passthrough"     , false        			  ,	"markup a markdown filename -filename write to stdout and exit." )
var ConfigurationTemplate 		 = flag.String( "config-template" , "node.yaml"  			  , "configuration file with golang template replacements" )
var ConfigurationFile     		 = flag.String( "config-file"     , "config.json"			  , "json format configuration file" )
var Format                		 = flag.Bool  ( "format"          , true         			  , "disable to compress spaces, the default is [readable] formatted output" )
var DumpCfg               		 = flag.String( "output"          , "output.json"			  , "write json configuration to named output file" )
var DumpTmpl              		 = flag.String( "dump-tmpl"       , "tmpl.yaml"  			  , "file to dump formatted output after template processing" )
var dump                  		 = flag.Bool  ( "dump"            , false        			  , "write formatted output after template processing" )
var DirectoryListingTemplateFile = "dir.html"
var DirectoryListingTemplateText = Load( DirectoryListingTemplateFile )

func Load( filename string ) ( *string ) {
    text, err := ioutil.ReadFile( filename )
    if err != nil {
        return nil
    }
    r := new( string )
    *r = string( text )
    return r
}

type MetadataMap map[string]string

type v0Cfg struct {
    CfgVersion            string             `json:CfgVersion:`
    HostIp                string             `json:HostIp:`
    HostName              string             `json:HostName:`
    HostSubnetBits        string             `json:HostSubnetBits:`
    HostCIDR              string             `json:HostCIDR:`
                                                                   
    Gateway               string             `json:Gateway:`
                                                                   
    DnsSearch             string             `json:DnsSearch:`
    Domain                string             `json:Domain:`
    Dns1                  string             `json:Dns1:`
    Dns2                  string             `json:Dns2:`

    // restricted access
    ClusterMgmtIp         string             `json:ClusterMgmtIp:`
    ClusterMgmtPort       string             `json:ClusterMgmtPort:`
    ClusterMgmtUrl        string             `json:ClusterMgmtUrl:`
                                                                   
    DockerIp              string             `json:DockerIp:`
    DockerSubnetBits      string             `json:DockerSubnetBits:`
    DockerCIDR            string             `json:DockerCIDR:`
    DockerMgmtPort        string             `json:DockerMgmtPort:`
    DockerMgmtUrl         string             `json:DockerMgmtUrl:`
                                                                   
    EtcdIp                string             `json:EtcdIp:`
    Etcdv1ClientPort      string             `json:Etcdv1ClientPort:`
    Etcdv1MgmtPort        string             `json:Etcdv1MgmtPort:`
    Etcdv2ClientPort      string             `json:Etcdv2ClientPort:`
    Etcdv2MgmtPort        string             `json:Etcdv2MgmtPort:`
    EtcdPeers             string             `json:EtcdPeers:`
    EtcdDiscoveryToken    string             `json:EtcdDiscoveryToken:`
    EtcdDiscoveryUrl      string             `json:EtcdDiscoveryUrl:`

    FleetMgmtUrl          string             `json:FleetMgmtUrl:`
                                                                   
    KubeletOptions        string             `json:KubeletOptions:`
    PrivilegedOption      string             `json:PrivilegedOption:`

    ProxyUrl              string             `json:ProxyUrl:`       

    Metadata              MetadataMap        `json:Metadata:`
}

type Cfg v0Cfg


var CfgVersion            = "v0"
var HostName              = "coreos-001"
var HostIp                = "10.2.5.65"
var HostSubnetBits        = "23" 
var HostCIDR              = HostIp + "/" + HostSubnetBits

var Gateway               = "10.2.3.1" 


var ClusterMgmtIp         = "10.2.3.19"
var ClusterMgmtPort       = "8080"
var ClusterMgmtUrl        = "http://" + ClusterMgmtIp + ":" + ClusterMgmtPort

var EtcdIp                = "10.2.3.19"
var Etcdv1ClientPort      = "4001"
var Etcdv1MgmtPort        = "7001"
var Etcdv2ClientPort      = "2379"
var Etcdv2MgmtPort        = "2380"
var EtcdPeers             = "http://" + EtcdIp + ":" + Etcdv1ClientPort

var EtcdDiscoveryToken    =  "4421819c7e708956af7ab67ddcd7ace3"
var EtcdDiscoveryUrl      = "http://" + ClusterMgmtIp + "/" + EtcdDiscoveryToken

var FleetMgmtUrl          = EtcdPeers

var DnsSearch             = "example.com"
var Domain                = "example.com"
var Dns1                  = "10.2.4.229"
var Dns2                  = "10.2.4.228"

// Some of this might go away with Calico or Flannel
var DockerIp              = "10.2.5.193"
var DockerSubnetBits      = "28"
var DockerCIDR            = DockerIp + "/" + DockerSubnetBits
var DockerMgmtPort        = "2375"
var DockerMgmtUrl         = "tcp://"+HostIp+":"+DockerMgmtPort
// Metadata

// Host Location
var DataCenter            = "sftlyr:dal01"

// should be discovered on boot -- client in get request header
var Platform              = "HP G8 12 Core" 
var MachineArchitecture   = "x86_64"        
var Engines               = "docker"

var Metadata              = map[string]string{ 
    "arch": MachineArchitecture,
    "datacenter" :DataCenter,
    "platform" :Platform,
    "engines" : Engines,
    "hostname" : HostName ,
    "mgmt" : "wkr" ,
    "mntr" : "false" ,
    "public_ip" : HostIp,
    "country": "US",
    "region": "central",
    "zone": "private",
    "role": "node",
}
var PrivilegedOption      = ""
var ProxyUrl              = "http://userid:proxypass@proxyhost.com:80"

var cfg                   = Cfg{ 
    CfgVersion            : CfgVersion ,
    HostIp                : HostIp ,
    HostName              : HostName ,
    HostSubnetBits        : HostSubnetBits ,
    HostCIDR              : HostCIDR ,
    Gateway               : Gateway ,

    DnsSearch             : DnsSearch ,
    Domain                : Domain ,       
    Dns1                  : Dns1 ,         
    Dns2                  : Dns2 ,         

    ClusterMgmtIp         : ClusterMgmtIp ,
    ClusterMgmtPort       : ClusterMgmtPort ,
    ClusterMgmtUrl        : ClusterMgmtUrl,

    DockerIp              : DockerIp,
    DockerSubnetBits      : DockerSubnetBits ,
    DockerCIDR            : DockerCIDR ,
    DockerMgmtPort        : DockerMgmtPort ,
    DockerMgmtUrl         : DockerMgmtUrl ,

    Metadata              : Metadata,

    EtcdIp                : EtcdIp,
    Etcdv1ClientPort      : Etcdv1ClientPort,
    Etcdv1MgmtPort        : Etcdv1MgmtPort,
    Etcdv2ClientPort      : Etcdv2ClientPort,
    Etcdv2MgmtPort        : Etcdv2MgmtPort,
    EtcdPeers             : EtcdPeers,
    EtcdDiscoveryToken    : EtcdDiscoveryToken,
    EtcdDiscoveryUrl      : EtcdDiscoveryUrl,

    FleetMgmtUrl          : FleetMgmtUrl ,

    KubeletOptions        : "--etcd_servers="+ClusterMgmtUrl ,
    PrivilegedOption      : PrivilegedOption ,

    ProxyUrl              : ProxyUrl,

}

func Commaize( m MetadataMap ) string {
    return m.Commaize()
}

func ( m MetadataMap ) Commaize() string {

    var text string

    var i    int    = 0

    sorted := make([]string, len(m))

    for key, _ := range m {
        sorted[i] = key
        i++
    }

    sort.Strings( sorted )

    for i, key := range sorted {
        if i > 0 {
            text += ","
        }
        text = text +  key + "=" + strings.ToLower( strings.Replace( m[key], " ", "-", -1 ) )
    }
    return text
}

var CfgFmap = template.FuncMap {
    "Commaize"    : Commaize,
}

var goTemplateRegex  = regexp.MustCompile(".*(\\{\\{.*\\}\\}).*")

func (cfg *Cfg) UpdateCfg( ip string ) {
	cfg.HostIp     = ip
	cfg.HostCIDR   = cfg.HostIp + "/" + cfg.HostSubnetBits
    for key, value := range cfg.Metadata {
		if key == "public_ip" {
			value = ip
		}
		cfg.Metadata[key] = strings.ToLower( strings.Replace( value, " ", "-", -1 ) )
		if debug { Info.Printf( " key " + key + " cfg.Metadata[key] " + cfg.Metadata[key] + "\n" ) }
    }
}

func (cfg *Cfg) LoadCfg() ( error ) {
    ConfigurationText      := Load( *ConfigurationFile )

    if ConfigurationText == nil {
        fmt.Fprintf( os.Stderr, "LoadCfg problem loading configuration [%s]\n", ConfigurationFile )
        return nil
    }

    err          := json.Unmarshal( []byte( *ConfigurationText ), cfg )

    if err != nil {
        fmt.Fprintf( os.Stderr, "LoadCfg problem unmarshalling configuration from ConfigurationFile[%s]\n", 
            ConfigurationText )
        return err
    }
    return nil
}

func ( cfg *Cfg ) CfgGen( ConfigurationFile string ) ( string ) {
    if cfg == nil {
        fmt.Fprintf( os.Stderr, "CfgGen error cfg argument error\n" )
        os.Exit( 1 )
    }
    configurationTemplate  := Load( *ConfigurationTemplate )
    
    if configurationTemplate != nil {

        runner, err := template.New("Configuration").Funcs( CfgFmap ).Parse( *configurationTemplate )
        if err != nil {
            fmt.Printf("Parse error: TemplateText[%s] %v\n", *configurationTemplate, err)
            return ""
        }

        writebuffer := bytes.NewBuffer( make([]byte, 0 ) ) //  []byte( *Configuration ) )
        writer      := bufio.NewWriter( writebuffer )
        err          = runner.Execute ( writer, cfg )

        if err != nil {
            fmt.Printf( "Template [%s] evaluation error Configuration file [%s]: %v\n", ConfigurationFile, *configurationTemplate, err )
            return ""
        }

        writer.Flush()
        return writebuffer.String()
    }
    return ""
}

func ( cfg Cfg ) Dump() {
    var err error
    var output []byte
    if *Format {
        output, err = json.MarshalIndent(cfg, "", "  ")

    } else {
        output, err = json.Marshal( cfg )
    }
    if err != nil {
        fmt.Fprintf( os.Stderr, "Error: unable to handle %v\n", cfg )
        os.Exit( 1 )
    }
    _, err = os.Stdout.Write( output )
    _, err = os.Stdout.Write( []byte("\n") )
}

func ( cfg Cfg ) String() string {
    var err error
    var output []byte
	output, err = json.Marshal( cfg )
    if err != nil {
        fmt.Fprintf( os.Stderr, "Error: unable to handle %v\n", cfg )
        os.Exit( 1 )
    }
    return string( output )
}
 
func exists( file string ) bool {
    _, err := os.Stat( file ); 
    return err == nil
}

type TemplateText string

func ( tmpl TemplateText ) Save( filename string ) {
    var output []byte =[]byte( tmpl )
	
	// Seed random number generator. 
	// Force repeatable sequences setting the pseudo arg true
	pseudo       := false
	Seed( pseudo )
	ok  		 := false
	outfilename  := filename 

	for i:=0; i<3; i++ {
		if ! exists( outfilename ) {
			ok = true
		} else {
			fmt.Fprintf( os.Stderr, "Refusing to overwrite existing file [%s]\n", outfilename )
			outfilename = filename + RandSeq(5) ;
			fmt.Fprintf( os.Stderr, "Attempting to redirect save to [%s]\n", outfilename )
		}
	}

	if ! ok {
		fmt.Fprintf( os.Stderr, "Failed to redirect write; exiting. . .\n" )
		os.Exit( 1 )
	}
	filename = outfilename
    var e error
    e    = ioutil.WriteFile( filename, output, 0644 )

    if e != nil {
        fmt.Printf("File save error: %v\n", e)
        os.Exit(1)
    }
}

func ( cfg Cfg ) Save( filename string ) {
    var err error
    var output []byte

    if exists( filename ) {
        fmt.Fprintf( os.Stderr, "Refusing to overwrite existing file [%s]\n", filename )
        os.Exit( 1 )
    }
    if *Format {
        output, err = json.MarshalIndent(cfg, "  ", "  ")

    } else {
        output, err = json.Marshal( cfg )
    }
    if err != nil {
        fmt.Fprintf( os.Stderr, "Error: unable to handle %v\n", cfg )
        os.Exit( 1 )
    }

    var e error
    e    = ioutil.WriteFile( filename, output, 0644 )
    if e != nil {
        fmt.Printf("File error: %v\n", e)
        os.Exit(1)
    }
}

func PassthroughWriter() {

    if *DumpCfg != "output.json" {
        cfg.Save( *DumpCfg )
    }

    if len( *ConfigurationFile ) == 0 {
        fmt.Fprintf( os.Stderr, "Configuration file not specified\n" )
        flag.PrintDefaults()
        os.Exit( 1 )
    }
	cfg := new( Cfg )
	if err := cfg.LoadCfg(); err == nil {
		cfg.UpdateCfg( "1.1.1.1" )
		fmt.Fprintf( os.Stdout, "%s\n", cfg.CfgGen( *ConfigurationTemplate ))
		if *dump {
			if len( *DumpTmpl ) > 0 {
				var logable TemplateText = TemplateText( cfg.CfgGen( *ConfigurationTemplate ) )
				logable.Save( *DumpTmpl )
			}
		}
	} else {
        fmt.Fprintf( os.Stderr, "cfg.LoadCfg error %s\n", err )
        flag.PrintDefaults()
        os.Exit( 1 )
	}
}

var NotFound = Load( "404.html" )
func errorHandler(w http.ResponseWriter, r *http.Request, status int ) {
    w.WriteHeader(status)
    if status == http.StatusNotFound {
		text := ""
		if NotFound != nil {
			text = *NotFound
			fmt.Fprint( w, text )
		} else {
			fmt.Fprint(w, "IP not provided in url[http://host. . .:port/ip-address]" )
		}
    }
}

var IpRegx     = "([0-9][0-9]?[0-9]?\\.[0-9][0-9]?[0-9]?\\.[0-9][0-9]?[0-9]?\\.[0-9][0-9]?[0-9]?)$"
var SubIpPath  = regexp.MustCompile("^/(config|status|plain|etcd|json|cloud\\-init)/"+ IpRegx +"$")
var RootIpPath = regexp.MustCompile("^/"+ IpRegx +"$")
var pathRegex  = regexp.MustCompile("^(/|config|status|plain|etcd|json|cloud\\-init/).*$")
var plainRegex = regexp.MustCompile("^(/plain/).*")

func MakeHandler( fn func( http.ResponseWriter, *http.Request, string, string, Arglist ), args Arglist ) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		Info.Printf( fmt.Sprintf( "Host: %-20s: Client : %-20s : URL.Path : [%s] : %s\n", r.Host, r.RemoteAddr, r.URL.Path, LogString( r ) ) )
        root     := RootIpPath.FindStringSubmatch( r.URL.Path )
		subpath  := SubIpPath .FindStringSubmatch( r.URL.Path )
		path, ip := "", ""

		if subpath != nil {
			if debug { Info.Printf( LogString( r ) + " subpath != nil url " + subpath[0] + ", " + subpath[1] + ", " + subpath[2] + "\n" ) }
			path, ip = subpath[1], subpath[2]
        } else if root != nil {
			if debug { Info.Printf( LogString( r ) + " root != nil url " + root[0] + "," + root[1] + "\n" ) }
			path = "/"
			ip   = root[1]
		} else {
			if debug { Info.Printf( LogString( r ) + " using default r.URL.Path " + r.URL.Path + "\n" ) }
			path = r.URL.Path
		}
		if debug { Info.Printf( LogString( r ) + " url " + r.URL.Path + " path " + path + " ip " + ip + "\n" ) }
	    if debug { Info.Printf( LogString( r ) + fmt.Sprintf(  " %v " , pathRegex.MatchString( path ) ) + "\n" ) }
		switch  {
		case len( path ) > 1:
			if debug { Info.Printf( LogString( r ) + " debug " + path + "\n" ) }
			fn( w, r, path, ip, args )
		default:
			if debug { Info.Printf( LogString( r ) + " /\n" ) }
			errorHandler( w, r, http.StatusNotFound )
		}
    }
}

func Handler( w http.ResponseWriter, r *http.Request, path, ip string, args Arglist ) {
	if debug { Info.Printf( LogString( r ) + " url " + r.URL.Path + " path " + path + " ip " + ip + " args " + fmt.Sprintf( "%v", args ) + "\n" ) }
	paths := plainRegex.FindStringSubmatch( path )
	if debug { Info.Printf( LogString( r ) + " paths " + fmt.Sprintf( "%v", paths ) + "\n" ) }
	switch {
	case paths != nil && len( paths ) > 1 : // "plain":
		a := strings.Split( path, "/" )
		if debug { Info.Printf( LogString( r ) + " debug " + r.URL.Path + " path " + path + " ip " + ip + " args " + fmt.Sprintf( "%v [%v]", args, a ) + "\n" ) }
		if len( a ) > 2 {
			if debug { Info.Printf( LogString( r ) + " debug " + fmt.Sprintf( "a[0] %v a[1] %v a[2] %v", a[0], a[1], a[2] ) + "\n" ) }
			path = strings.Join( a[2:], "/" )
			if debug { Info.Printf( LogString( r ) + " debug " + fmt.Sprintf( "[%v], paths[%v] path[%v]", a, paths, path ) + "\n" ) }
			HandleRaw( w, r, &path )
		} else {
			errorHandler( w, r, http.StatusNotFound )
		}
	default:
		// remove path separators where ip is found
		path = strings.Replace( path, "/", "", -1 )
		switch path {
		case "" :
			HandleRoot( w, r, ip, args )
		case "json":
			if ip == "" {
				HandleRaw( w, r, ConfigurationFile )
			} else {
				HandleCfgModify( w, r, ip, ConfigurationFile )
			}
		case "config" :
			if ip == "" {
				errorHandler( w, r, http.StatusNotFound )
			} else {
				HandleRoot( w, r, ip, args )
			}
		case "cloud-init":
			if ip == "" {
				HandleRaw( w, r, ConfigurationTemplate )
			} else {
				HandleRoot( w, r, ip, args )
			}
		case "etcd":
			HandleEtcd( w, r, ip )
		}
	}
}

func etcdPeers() ( []string ) {
	peers := make( []string, 1 )
	if strings.Index( *EtcdPeerList, "," ) >= 0 {
		for _, peer := range strings.Split( *EtcdPeerList, ",") {
			peers  = append( peers, peer )
		}
	} else {
		peers  = append( peers, *EtcdPeerList )
	}
	return peers
} 

func HandleRoot( w http.ResponseWriter, r *http.Request, ip string, args Arglist ) {
	cfg := new( Cfg )
    
    if err := cfg.LoadCfg(); err == nil {

		cfg.UpdateCfg( ip )
		text := cfg.CfgGen( *ConfigurationTemplate )
		if text == "" {
			http.NotFound( w, r )
			return 
		}

		w.Write( []byte( template.HTML( text ) ) )

		c := etcd.NewClient( etcdPeers() )
		var ttl uint64 = 1800 

		c.CreateDir( "/config/by_ip/", ttl * 2 )
		c.Set( "/config/by_ip/"+cfg.HostIp, cfg.String(), ttl )
	} else {
        fmt.Fprintf( os.Stderr, "Error loading configuration from file %v %s\n", err, *ConfigurationFile )
        os.Exit( 1 )
    }

}

func HandleRaw( w http.ResponseWriter, r *http.Request, rawName *string ) {
	if debug { Info.Printf( LogString( r ) + " url " + r.URL.Path + " rawName " + *rawName +"\n" ) }
	if rawName != nil {
		text := Load( *rawName )
		if text != nil {
			for found := strings.Index( *text, "%s" ) >= 0; found; found = strings.Index( *text, "%s" ) >= 0 {
				*text = fmt.Sprintf( "%s", r.Host )
			}
			w.Write( []byte( template.HTML( *text ) ) )
		}
	}
}

func HandleCfgModify( w http.ResponseWriter, r *http.Request, ip string, rawName * string ) {
    cfg := new( Cfg )
    if cfg == nil {
        fmt.Fprintf( os.Stderr, "Unable to create Cfg object cfg\n" )
        os.Exit( 1 )
    }

	if err := cfg.LoadCfg(); err == nil {
		cfg.UpdateCfg( ip )

		text, _ := json.MarshalIndent(cfg, "", "  ")
		w.Write( []byte( template.HTML( text ) ) )
	}
}

func HandleEtcd( w http.ResponseWriter, r *http.Request, ip string ) {

	c := etcd.NewClient( etcdPeers() )
	sorted, recurse := false, false
	if ip == "" {
		sorted, recurse = true, true
		jsoncfg, err := c.Get( "/config/by_ip/"+ip, sorted, recurse )
		type CfgX struct{
			EtcdPath              string             `json:EtcdPath:`
			Ip                    string             `json:Ip:`
			Cfg*                  Cfg                `json:Cfg:`
		}

		if err == nil {
			for _, y := range jsoncfg.Node.Nodes {
				key    := y.Key
				value  := y.Value
				var cfgX CfgX
				var cfg *Cfg = new(Cfg)
				err  := json.Unmarshal( []byte( value ), cfg )
				if err == nil {
					dirs := strings.Split( key, "/" )
					cfgX.EtcdPath  = key
					cfgX.Ip  	   = dirs[len(dirs)-1]
					cfgX.Cfg 	   = cfg
					text, err   := json.MarshalIndent( cfgX, "", "  ")
					if err == nil {
						w.Write( []byte( template.HTML( string( text ) + "\n\n" ) ) )
					}
				}
			}
		} else {
			w.Write( []byte( template.HTML( fmt.Sprintf( "%s", err ) ) ) )
		}
	} else {
		jsoncfg, err := c.Get( "/config/by_ip/"+ip, sorted, recurse )
		if err == nil {
			var cfg Cfg
			_        = json.Unmarshal( []byte(jsoncfg.Node.Value), &cfg )
			text, _ := json.MarshalIndent( cfg, "", "  ")
			w.Write( []byte( template.HTML( text ) ) )
		} else {
			w.Write( []byte( template.HTML( fmt.Sprintf( "%s", err ) ) ) )
		}
	}
}

// https://blog.golang.org/context/userip/userip.go
// FromRequest extracts the user IP address from req, if present.
func FromRequest(req *http.Request) (net.IP, error) {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("userip: %q is not IP:port", req.RemoteAddr)
	}

	userIP := net.ParseIP(ip)
	if userIP == nil {
		return nil, fmt.Errorf("userip: %q is not IP:port", req.RemoteAddr)
	}
	return userIP, nil
}

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// userIPkey is the context key for the user IP address.  Its value of zero is
// arbitrary.  If this package defined other context keys, they would have
// different integer values.
const userIPKey key = 0

// NewContext returns a new Context carrying userIP.
func NewContext(ctx context.Context, userIP net.IP) context.Context {
	return context.WithValue(ctx, userIPKey, userIP)
}

// FromContext extracts the user IP address from ctx, if present.
func FromContext(ctx context.Context) (net.IP, bool) {
	// ctx.Value returns nil if ctx has no value for the key;
	// the net.IP type assertion returns ok=false for nil.
	userIP, ok := ctx.Value(userIPKey).(net.IP)
	return userIP, ok
}

func main() {
    if *Passthrough {
        PassthroughWriter()
        os.Exit(0)
    }


    fmt.Println( "This can be configured with the following environment variables." )
    fmt.Println( "CFGGEN_WEBROOT to set the path to serve. The default path is the current directory." )
    fmt.Println( "CFGGEN_PORT to set the port to serve on. The default port is 8080" )
    fmt.Println( "CFGGEN_HOST to set the host interface to serve. The default is all." )

    if len( CfgGenWebRoot ) == 0 || strings.Contains( CfgGenWebRoot, "../") {
        fmt.Println( "CFGGEN_WEBROOT not set or using .. to subvert paths, using the default instead" )
        CfgGenWebRoot = "."
    } else {
        fmt.Println( "CFGGEN_WEBROOT=" + CfgGenWebRoot )
    }
    if len( CfgGenPort ) == 0 {
        fmt.Println( "CFGGEN_PORT not set, using 8080" )
        CfgGenPort = "8080"
    } else {
        fmt.Println( "CFGGEN_PORT=" + CfgGenPort )
    }
    if len( CfgGenHost ) == 0 {
        fmt.Println( "CFGGEN_HOST not set, default bind all" )
        CfgGenHost = "0.0.0.0"
    } else {
        fmt.Println( "CFGGEN_HOST=" + CfgGenHost )
    }
    listen := CfgGenHost + ":" + CfgGenPort 

    fmt.Println( "WEBROOT to serve  " + ": " + CfgGenWebRoot )
    fmt.Println( "PORT on which     " + ": " + CfgGenPort )
    fmt.Println( "HOST interface    " + ": " + CfgGenHost )
    fmt.Println( "listening on      " + listen + " and serving " + CfgGenWebRoot )

    // RootFolder      := &webRoot
    root_name       :=  "root"
    config_name     :=  "config"
    plain_name      :=  "plain"  
    cloud_init_name :=  "cloud-init"  
    etcd_name       :=  "etcd"  
    json_name       :=  "json"  

    root            := Arglist{ nil, &root_name,  		 &CfgGenWebRoot, listen, -1, -1, }
    json_cfg        := Arglist{ nil, &json_name,  		 &CfgGenWebRoot, listen, -1, -1, }
    etcd_cfg        := Arglist{ nil, &etcd_name,  		 &CfgGenWebRoot, listen, -1, -1, }
    config          := Arglist{ nil, &config_name,		 &CfgGenWebRoot, listen, -1, -1, }
    plain           := Arglist{ nil, &plain_name ,		 &CfgGenWebRoot, listen, -1, -1, }
    cloud_init      := Arglist{ nil, &cloud_init_name,   &CfgGenWebRoot, listen, -1, -1, }

	mux := http.NewServeMux()
    mux.HandleFunc( "/"           	,    MakeHandler( Handler, root   ) )
    mux.HandleFunc( "/plain/"     	,    MakeHandler( Handler, plain  ) )
    mux.HandleFunc( "/cloud-init/"  ,    MakeHandler( Handler, cloud_init  ) )
    mux.HandleFunc( "/config/"    	,    MakeHandler( Handler, config ) )
    mux.HandleFunc( "/etcd/"      	,    MakeHandler( Handler, etcd_cfg ) )
    mux.HandleFunc( "/json/"      	,    MakeHandler( Handler, json_cfg ) )

	server := &http.Server{
		Addr:           listen,
		Handler:        mux,
		ReadTimeout:    time.Duration(1000 * int64(time.Second)),
		WriteTimeout:   time.Duration(1000 * int64(time.Second)),
		MaxHeaderBytes: 1 << 20,
	}
	server.ListenAndServe()
}

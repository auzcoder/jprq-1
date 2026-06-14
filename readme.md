<p align="center">
    <h1 align="center">⚡ SpeedTunnel</h1>
    <p align="center">Fast and secure tunneling service for exposing local servers to the internet</p>
</p>

## What's SpeedTunnel?

- SpeedTunnel is a free and open tool for exposing local servers to public network (the internet)
- It can expose TCP protocols, such as HTTP, SSH, etc. Any server!

---

## How to install

<details>
<summary>Windows</summary>

### Manual Installation

1. Install the latest <a href='https://github.com/AUZ-DEV/speedtunnel/releases'>release</a> of SpeedTunnel<br>
2. Place the file where it is convenient for you<br><br>
   <i>(At this point, you can use the program, but you will need to manually call the <code>.exe</code> file)</i><br>
3. Create <b>speedtunnel.bat</b> file so we can use the "speedtunnel" keyword to call the <b>.exe</b> file<br>

    ```bash
    @echo off
    "C:\Exact\Path\To\File\speedtunnel-windows-386.exe" %*
    ```

4. Add to the environment variable "PATH" the path to the folder where we created .bat file

<p align='center'><b>Congratulations!</b> You can check if everything is working with the speedtunnel command in CMD</p>
<hr>

</details>

<details>
    <summary> MacOS and Linux</summary>

```bash
$ curl -fsSL https://speedtunnel.io/install.sh | sudo bash
```

</details>

## How to use

First obtain auth token, then

```bash
speedtunnel auth <your-auth-token>
```

Replace 8000 with the port you want to expose

```bash
speedtunnel http 8000
```

For exposing any TCP servers, such as SSH

```bash
speedtunnel tcp 22
```

For using custom subdomains

```bash
speedtunnel http 3000 -s <custom-name>
```

For using SpeedTunnel debugger (with v1.0 or higher)

```bash
speedtunnel http 3000 --debug
```

Serve static files using built-in HTTP Server

```bash
speedtunnel serve .
```

Serve on a different domain using CNAME

```bash
speedtunnel http 3000 --cname example.com
```

Press Ctrl+C to stop it

---

## Developer

**Davlatov Abdulhafiz**

- 📧 Email: auz.offical@gmail.com
- 📱 Phone: +998906960010

## Version

v1.0.0

## License

Open Source

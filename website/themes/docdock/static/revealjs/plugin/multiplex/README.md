# Multiplex Plugin

The multiplex plugin allows your audience to view the slides of the reveal.js presentation you are controlling on their own phone, tablet or laptop. As the master presentation navigates the slides, all client presentations will update in real time. See a demo at <https://reveal-multiplex.glitch.me/>).

The multiplex plugin needs the following three things to operate:

1. [Master presentation](#master-presentation) that has control
2. [Client presentations](#client-presentation) that follow the master
3. [Socket.io server](#socketio-server) to broadcast events from the master to the clients

## Getting Started

1. Navigate to your reveal.js folder
1. `npm install reveal-multiplex`
1. `node node_modules/reveal-multiplex`
1. Follow the instructions below to set up your master and client

### Hosted Server

In the following examples we configure the multiplex plugin to connect to a socket.io server hosted at <https://reveal-multiplex.glitch.me/>. You are very welcome to point your presentations at this socket.io server, but availability and stability are not guaranteed. **For anything mission critical we recommend running your own multiplex server**.

## Master Presentation

Served from a static file server accessible only to the presenter. This need only be on your (the presenter's) computer. (It's safer to run the master presentation from your own computer, so if the venue's Internet goes down it doesn't stop the show.) An example would be to execute the following commands in the directory of your master presentation:

1. `npm install node-static`
2. `static`

If you want to use the speaker notes plugin with your master presentation then make sure you have the speaker notes plugin configured correctly along with the configuration shown below, then execute `node node_modules/reveal-notes-server` in the directory of your master presentation. The configuration below will cause it to connect to the socket.io server as a master, as well as launch your speaker-notes/static-file server.

You can then access your master presentation at `http://localhost:1948`

Example configuration:

```javascript
Reveal.initialize({
  multiplex: {
    // Example values. To generate your own, see the socket.io server instructions.
    secret: '13652805320794272084', // Obtained from the socket.io server. Gives this (the master) control of the presentation
    id: '1ea875674b17ca76', // Obtained from socket.io server
    url: 'https://reveal-multiplex.glitch.me/' // Location of socket.io server
  },

  // Don't forget to add the dependencies
  dependencies: [
    { src: 'https://reveal-multiplex.glitch.me/socket.io/socket.io.js', async: true },
    { src: 'https://reveal-multiplex.glitch.me/master.js', async: true },

    // and if you want speaker notes
    { src: 'https://unpkg.com/reveal-notes-server/client.js', async: true }
  ]
});
```

## Client Presentation

Served from a publicly accessible static file server such as GitHub Pages or Amazon S3. Your audience can then access the client presentation via `https://example.com/path/to/presentation/client/index.html`, with the configuration below causing them to connect to the socket.io server as clients.

Example configuration:

```javascript
Reveal.initialize({
  multiplex: {
    // Example values. To generate your own, see the socket.io server instructions.
    secret: null, // null so the clients do not have control of the master presentation
    id: '1ea875674b17ca76', // id, obtained from socket.io server
    url: 'https://reveal-multiplex.glitch.me/' // Location of socket.io server
  },

  // Don't forget to add the dependencies
  dependencies: [
    { src: 'https://reveal-multiplex.glitch.me/socket.io/socket.io.js', async: true },
    { src: 'https://reveal-multiplex.glitch.me/client.js', async: true }
  ]
});
```

## Socket.io Server

Server that receives the `slideChanged` events from the master presentation and broadcasts them out to the connected client presentations. This needs to be publicly accessible. You can run your own socket.io server with `node node_modules/reveal-multiplex`.

Or you can use the socket.io server at <https://reveal-multiplex.glitch.me/>.

You'll need to generate a unique secret and token pair for your master and client presentations. To do so, visit `https://example.com/token`, where `https://example.com` is the location of your socket.io server. Or if you're going to use the socket.io server at <https://reveal-multiplex.glitch.me/>, visit [https://reveal-multiplex.glitch.me/token](https://reveal-multiplex.glitch.me/token).

### socket.io server as file static server

The socket.io server can play the role of static file server for your client presentation, as in the example at <https://reveal-multiplex.glitch.me/>. (Open <https://reveal-multiplex.glitch.me/> in two browsers. Navigate through the slides on one, and the other will update to match.)

Example configuration:

```javascript
Reveal.initialize({
  multiplex: {
    // Example values. To generate your own, see the socket.io server instructions.
    secret: null, // null so the clients do not have control of the master presentation
    id: '1ea875674b17ca76', // id, obtained from socket.io server
    url: 'example.com:80' // Location of your socket.io server
  },

  // Don't forget to add the dependencies
  dependencies: [
    { src: 'https://reveal-multiplex.glitch.me/socket.io/socket.io.js', async: true },
    { src: 'https://reveal-multiplex.glitch.me/client.js', async: true }
  ]
```

It can also play the role of static file server for your master presentation and client presentations at the same time (as long as you don't want to use speaker notes). (Open <https://reveal-multiplex.glitch.me/> in two browsers. Navigate through the slides on one, and the other will update to match. Navigate through the slides on the second, and the first will update to match.) This is probably not desirable, because you don't want your audience to mess with your slides while you're presenting. ;)

Example configuration:

```javascript
Reveal.initialize({
  multiplex: {
    // Example values. To generate your own, see the socket.io server instructions.
    secret: '13652805320794272084', // Obtained from the socket.io server. Gives this (the master) control of the presentation
    id: '1ea875674b17ca76', // Obtained from socket.io server
    url: 'example.com:80' // Location of your socket.io server
  },

  // Don't forget to add the dependencies
  dependencies: [
    { src: 'https://reveal-multiplex.glitch.me/socket.io/socket.io.js', async: true },
    { src: 'https://reveal-multiplex.glitch.me/master.js', async: true },
    { src: 'https://reveal-multiplex.glitch.me/client.js', async: true }
  ]
});
```
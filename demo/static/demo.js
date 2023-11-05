/* eslint-env browser */

const pc = new RTCPeerConnection({
    iceServers: [{
        urls: 'stun:stun.l.google.com:19302'
    }]
})

pc.ontrack = function (event) {
    const el = document.createElement(event.track.kind)
    el.srcObject = event.streams[0]
    el.autoplay = true
    el.muted = true
    el.controls = true

    document.getElementById('remoteVideos').appendChild(el)
}

const offerReadyPromise = new Promise((resolve, reject) => {
    pc.onicecandidate = event => {
        if (event.candidate == null) {
            resolve()
        }
    }
})

const socket = new WebSocket("ws://" + window.location.hostname + "/watch");
const socketOpenPromise = new Promise((resolve, reject) => {
    socket.onopen = event => {
        resolve()
    }
});

Promise.all([offerReadyPromise, socketOpenPromise]).then(value => {
    socket.send(JSON.stringify(pc.localDescription))
})

socket.addEventListener("message", (event) => {
    pc.setRemoteDescription(JSON.parse(event.data)).then(ev => {
        pc.createAnswer().then(d => pc.setLocalDescription(d))
    })
})

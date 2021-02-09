// Create peer conn
const pc = new RTCPeerConnection({
    iceServers: [
        {
            urls: "stun:stun.l.google.com:19302",
        },
    ],
});

pc.oniceconnectionstatechange = (e) => {
    console.log("connection state change", pc.iceConnectionState);
};
pc.onicecandidate = (event) => {
    if (event.candidate === null) {
        document.getElementById("localSessionDescription").value = btoa(
            JSON.stringify(pc.localDescription)
        );
    }
};

pc.onnegotiationneeded = (e) =>
    pc
        .createOffer()
        .then((d) => {
            console.log('offer');
            console.log(d);
            pc.setLocalDescription(d);
        })
        .catch(console.error);

pc.ontrack = (event) => {
    console.log("Got track event", event);
    let video = document.createElement("video");
    video.srcObject = event.streams[0];
    video.autoplay = true;
    video.width = "500";
    let label = document.createElement("div");
    label.textContent = event.streams[0].id;
    document.getElementById("serverVideos").appendChild(label);
    document.getElementById("serverVideos").appendChild(video);
};

navigator.mediaDevices
    .getUserMedia({
        video: {
            width: 1280,
            height: 720,
            frameRate: 24,
        },
        audio: true,
    })
    .then((stream) => {
        const videoTrack = stream.getVideoTracks()[0];
        if (!videoTrack){
            alert("VideoTrack is null");
            return;
        }
        document.getElementById("browserVideo").srcObject = stream;
        pc.addTransceiver(videoTrack, {
            streams: [stream],
            direction: "sendonly",
            sendEncodings: [
                // for firefox order matters... first high resolution, then scaled resolutions...
                {
                    rid: "f",
                    maxBitrate:2_500_000,
                    maxFramerate: 30.0,
                },
                {
                    rid: "h",
                    scaleResolutionDownBy: 2.0,
                    maxBitrate: 500_000,
                    maxFramerate: 30.0,
                },
                {
                    rid: "q",
                    scaleResolutionDownBy: 4.0,

                    maxBitrate: 150_000,
                    maxFramerate: 15.0,
                },
            ],
        });
        const audioTrack = stream.getAudioTracks()[0];
        if(!audioTrack) {
            alert("audioTrack is null");
            return;
        }
        pc.addTrack(audioTrack, stream);
    });

window.startSession = () => {
    const sd = document.getElementById("remoteSessionDescription").value;
    if (sd === "") {
        return alert("Session Description must not be empty");
    }

    try {
        console.log("answer", JSON.parse(atob(sd)));
        pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))));
    } catch (e) {
        alert(e);
    }
};

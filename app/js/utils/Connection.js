let conn;

class Connection {
    static setUp() {
        console.log('Initializing websockets...');
        conn = new WebSocket(`ws://localhost:5874/ws?lastMod=143918dd9ce16851`);
        conn.onclose = e => {
            console.log('Connection closed');
        };
    }

    static onMessage(callback) {
        conn.onmessage = event => {
            const data = JSON.parse(event.data);
            callback(data);
        };
    }

    static sendMessage(newPackageData) {
        conn.send(JSON.stringify(newPackageData));
    }
}

export default Connection;
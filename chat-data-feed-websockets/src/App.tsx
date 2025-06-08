import { useEffect, useState } from 'react';
import './App.css';

const App = () => {
  const [isConnected, setIsConnected] = useState(false)
  const [messages, setMessages] = useState<string[]>([])
  const [socket, setSocket] = useState<WebSocket | null>(null)


  useEffect(() => {
    const socket = new WebSocket("ws://localhost:4000/ws")
    console.log({socket})

    socket.onmessage = event => {
      console.log(`received from the server: ${event.data}`)
      setMessages(prev => [...prev, event.data])
    }

    socket.onopen = () => {
      console.log("connected to the server")
      setIsConnected(true)
      setSocket(socket)
    }


    return () => {
      socket.close()
    }
  }, [])

  useEffect(() => {
    let socket = new WebSocket("ws://localhost:4000/orderbookfeed")
    socket.onmessage = event => {
      console.log(`[orderbook feedback] - received from the server: ${event.data}`)
    }

    return () => {
      socket.close()
    }
  }, [])


  return (
    <div className="content">
      <h1>Websocket with golang</h1>
      <span>{isConnected ? "Connected" : "Disconnected"}</span>
      <h2>Messages:</h2>
      <div style={{
        maxWidth: "500px",
        margin: "0 auto",
      }}>

      <ul>
        {messages.map((message, index)=> (
          <li key={`${index}-${message}`}>
            {message}
          </li>
        ))}
      </ul>
      </div>
      <div style={{
        maxWidth: "500px",
        margin: "0 auto",
      }}>
        <form onSubmit={event => {
          event.preventDefault()
          const formData = new FormData(event.target as HTMLFormElement)
          const message = formData.get("message") as string | null
          console.log({message, socket})
          if (message && socket) {

            socket.send(message)
          }
        }}>

        <input type="text" placeholder='message' required name='message' />
        <button type='submit'>Send</button>
        </form>
      </div>
    </div>
  );
};

export default App;

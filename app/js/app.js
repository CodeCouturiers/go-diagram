import React from 'react';
import { createRoot } from 'react-dom/client';
import { Provider } from 'react-redux';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import { configureStore } from '@reduxjs/toolkit';

// Import the pages
import HomePage from './components/pages/HomePage';
import ReadmePage from './components/pages/ReadmePage';
import NotFoundPage from './components/pages/NotFound';
import App from './components/App';

// Import the CSS file, which HtmlWebpackPlugin transfers to the build folder
import '../css/index.styl';

// Create the store with the redux-thunk middleware, which allows us
// to do asynchronous things in the actions
import rootReducer from './reducers/rootReducer';
import { createBrowserHistory } from 'history';

const store = configureStore({
    reducer: rootReducer,
    middleware: (getDefaultMiddleware) => getDefaultMiddleware(),
})


// Make reducers hot reloadable, see http://stackoverflow.com/questions/34243684/make-redux-reducers-and-other-non-components-hot-loadable
if (module.hot) {
    module.hot.accept('./reducers/rootReducer', () => {
        const nextRootReducer = require('./reducers/rootReducer').default;
        store.replaceReducer(nextRootReducer);
    });
}

const history = createBrowserHistory();

// Create a root element and render the app
const rootElement = document.getElementById('ReactApp');
const root = createRoot(rootElement);
root.render(
    <Provider store={store}>
        <Router history={history}>
            <App>
                <Routes>
                    <Route exact path="/" element={<HomePage />} />
                    <Route path="/readme" element={<ReadmePage />} />
                    <Route path="*" element={<NotFoundPage />} />
                </Routes>
            </App>
        </Router>
    </Provider>
);
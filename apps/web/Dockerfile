FROM node:24.22.0-bookworm-slim
LABEL io.syntelle.project="syntelle"
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]

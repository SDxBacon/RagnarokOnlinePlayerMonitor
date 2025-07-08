import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

import { ragnarok } from "../../wailsjs/go/models";

export enum TablePlayersMode {
  Number = "number",
  Status = "status",
}

// Maps the number of players to a status text
// certain values:
// - 3 = "爆滿"
//
// Others are guessed based on the context ( ´Д`)y━･~~
export const mapPlayersToStatusText = (players: number): string => {
  switch (players) {
    case 1:
      return "順暢";
    case 2:
      return "擁擠";
    case 3:
      return "爆滿";
    default:
      return `Unknown (${players})`;
  }
};

interface CharacterServerTableProps {
  data: ragnarok.CharacterServerInfo[];
  playersMode?: TablePlayersMode;
}

const CharacterServerTable = (props: CharacterServerTableProps) => {
  const { data, playersMode = TablePlayersMode.Number } = props;

  const isPlayersInNumberMode = playersMode === TablePlayersMode.Number;

  return (
    <Table className="text-left select-auto">
      <TableHeader className="bg-chart-4">
        <TableRow>
          <TableHead>Server</TableHead>
          <TableHead>Address</TableHead>
          <TableHead>{isPlayersInNumberMode ? "Players" : "Status"}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {data.map((item, index) => (
          <TableRow key={index} className="hover:bg-primary">
            <TableCell className="font-medium">{item.Name}</TableCell>
            <TableCell>{item.Url}</TableCell>
            <TableCell>
              {isPlayersInNumberMode
                ? item.Players
                : mapPlayersToStatusText(item.Players)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
};

export default CharacterServerTable;
